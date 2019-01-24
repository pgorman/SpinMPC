// SpinMPC is a music player client for mpd.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
)

type Configuration struct {
	Debug bool `json:"debug"`
	MPD   struct {
		Address  string `json:"address"`
		Port     string `json:"port"`
		Password string `json:"password"`
		Kill     string `json:"kill"`
	} `json:"mpd"`
	Web struct {
		Address  string `json:"address"`
		Port     string `json:"port"`
		Password string `json:"password"`
		Root     string `json:"root"`
		Search   string `json:"search"`
	} `json:"web"`
}

var conf = Configuration{}

func init() {
	cf := flag.String("c", "/etc/spinmpc.conf", "Specify the full path to the configuration file.")
	flag.BoolVar(&conf.Debug, "d", false, "Turn on debugging messages.")
	flag.StringVar(&conf.MPD.Address, "mdpaddr", "127.0.0.1", "Specify the IP address where MPD listens.")
	flag.StringVar(&conf.MPD.Port, "mdpport", "6600", "Specify the port on which MPD listens.")
	flag.StringVar(&conf.MPD.Password, "mdppass", "", "Specify password required by MPD (if any).")
	flag.StringVar(&conf.MPD.Kill, "mdpkill", "/usr/bin/mpd --kill", "Specify the command to kill MPD.")
	flag.StringVar(&conf.Web.Address, "webaddr", "127.0.0.1", "Specify the IP address where SpinMPC serves its web interface.")
	flag.StringVar(&conf.Web.Port, "webport", "8870", "Specify the port on which SpinMPC serves its web interface.")
	flag.StringVar(&conf.Web.Password, "webpass", "", "Password to require for access to SpinMPC's web interface.")
	flag.StringVar(&conf.Web.Root, "webroot", "./", "Directory from which to serve SpinMPC's web documents.")
	flag.StringVar(&conf.Web.Search, "websearch", "https://duckduckgo.com/?q=", "Set base URL for web searches.")
	flag.Parse()

	f, err := os.Open(*cf)
	if err != nil {
		log.Printf("can't open config file %s: ", *cf, err)
	}
	defer f.Close()
	if err == nil {
		decoder := json.NewDecoder(f)
		err = decoder.Decode(&conf)
		if err != nil && conf.Debug {
			log.Println("WARN: can't decode JSON in %s", *cf, err)
		}
	}

	if conf.Debug {
		log.Println("INFO: configured debug mode:", conf.Debug)
		log.Println("INFO: configured MPD address:", conf.MPD.Address)
		log.Println("INFO: configured MPD port:", conf.MPD.Port)
		if conf.MPD.Password == "" {
			log.Println("INFO: configured MPD password: [none]")
		} else {
			log.Println("INFO: configured MPD password: ****************************")
		}
		if conf.Web.Address == "" {
			log.Println("INFO: configured web address: [all]")
		} else {
			log.Println("INFO: configured web address:", conf.Web.Address)
		}
		log.Println("INFO: configured web port:", conf.Web.Port)
		if conf.Web.Password == "" {
			log.Println("INFO: configured web password: [none]")
		} else {
			log.Println("INFO: configured web password: ****************************")
		}
		log.Println("INFO: configured web root:", conf.Web.Root)
		log.Println("INFO: configured web search base:", conf.Web.Search)
	}
}

// ConnectMPD connects to the music player daemon.
func ConnectMPD(conf *Configuration) *mpd.Client {
	conn, err := mpd.Dial("tcp", conf.MPD.Address+":"+conf.MPD.Port)
	if err != nil {
		log.Fatal("ERROR: can't connect to MPD: ", err)
	}
	if conf.Debug {
		log.Println("INFO: successfully connected to MPD")
		status, err := conn.Status()
		if err != nil {
			log.Println("WARN: can't get MPD status: ", err)
		}
		log.Println("INFO: checking initial MPD status...")
		for k, v := range status {
			log.Println("INFO: MPD status:", k, v)
		}
		stats, err := conn.Stats()
		if err != nil {
			log.Println("WARN: can't get MPD stats: ", err)
		}
		log.Println("INFO: fetching MPD stats...")
		for k, v := range stats {
			log.Println("INFO: MPD stat:", k, v)
		}
	}
	return conn
}

// FillPlaylist populates an empty default playlist with all files in the database.
func FillPlaylist(conn *mpd.Client) error {
	var err error
	songs, err := conn.PlaylistInfo(-1, -1)
	if err != nil {
		err = fmt.Errorf("WARN: failed to get current playlist: &v", err)
	}
	// Don't clobber the current playlist if it's already populated!
	if len(songs) > 0 {
		if conf.Debug {
			log.Println("INFO: Playlist already populated. Abandoning 'fillPlaylist'.")
		}
		return err
	}

	err = conn.Clear()
	if err != nil {
		err = fmt.Errorf("WARN: failed to clear playlist: %v", err)
	}
	songs, err = conn.ListAllInfo("/")
	if err != nil {
		err = fmt.Errorf("WARN: failed to get song info: %v", err)
	}
	for _, s := range songs {
		err = conn.Add(s["file"])
		if err != nil {
			err = fmt.Errorf("WARN: can't add file to playlist: %v", err)
		}
	}
	StartStop(conn)
	return err
}

// KeepAlive keeps our connection to MPD open.
func KeepAlive(conn **mpd.Client, conf *Configuration) {
	retries := 0
	for {
		c := *conn
		err := c.Ping()
		if err != nil {
			log.Println("WARN: can't ping MPD: ", err)
			retries++
			if retries > 3 {
				Reconnect(conn, conf)
				retries = 0
			}
		}
		time.Sleep(time.Second * 5)
	}
}

// Playlists gets a list of playlists from MPD.
func Playlists(conn *mpd.Client) ([]string, error) {
	pa, err := conn.ListPlaylists()
	if err != nil {
		err = fmt.Errorf("WARN: failed to get playlists from MPD: %v", err)
	}
	playlists := make([]string, len(pa))
	for i := 0; i < len(pa); i++ {
		playlists[i] = pa[i]["playlist"]
	}
	return playlists, err
}

// Reconnect closes the current connection to MPD and opens a new one.
func Reconnect(conn **mpd.Client, conf *Configuration) error {
	var err error
	if conf.Debug {
		log.Println("INFO: reconnecting to MPD.")
	}
	oldc := *conn
	*conn = ConnectMPD(conf)
	oldc.Close()
	// TODO Make ConnectMPD returns errors, and return them in turn.
	return err
}

// SearchURL constructs a URL to web search a song.
func SearchURL(conf *Configuration, song map[string]string) string {
	q := url.QueryEscape(strings.Join([]string{"\"", song["Artist"], "\" \"", song["Title"], "\" \"", song["Album"], "\""}, ""))
	return strings.Join([]string{conf.Web.Search, q}, "")
}

// StartStop starts and immiately stop, so there's a song populated in "status". Why is this needed?
func StartStop(conn *mpd.Client) error {
	var err error
	err = conn.Play(-1)
	if err != nil {
		err = fmt.Errorf("WARN: failed to stop playback: %v", err)
	}
	err = conn.Stop()
	if err != nil {
		fmt.Errorf("WARN: failed to stop playback: %v", err)
	}
	return err
}

func main() {
	// This pointer to the pointer to the mpd.Client ugliness lets us
	// connect a new client if the old one dies by substituting in the new
	// pointer.
	var conn **mpd.Client
	c := ConnectMPD(&conf)
	conn = &c
	defer (*conn).Close()
	go KeepAlive(conn, &conf)
	err := FillPlaylist(*conn)
	if err != nil {
		log.Println(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(conf.Web.Root, "index.html"))
	})

	http.HandleFunc("/api/v1/allsongs", func(w http.ResponseWriter, r *http.Request) {
		err := (*conn).Clear()
		if err != nil {
			log.Println("WARN: failed to clear play queue: ", err)
		}
		FillPlaylist(*conn)
	})

	http.HandleFunc("/api/v1/clearqueue", func(w http.ResponseWriter, r *http.Request) {
		err := (*conn).Clear()
		if err != nil {
			log.Println("WARN: failed to clear play queue: ", err)
		}
	})

	http.HandleFunc("/api/v1/currentsong", func(w http.ResponseWriter, r *http.Request) {
		s, err := (*conn).CurrentSong()
		if err != nil {
			log.Println("WARN: failed to get current song info: ", err)
		}
		s["SearchURL"] = SearchURL(&conf, s)
		json.NewEncoder(w).Encode(s)
	})

	http.HandleFunc("/api/v1/killmpd", func(w http.ResponseWriter, r *http.Request) {
		if conf.Debug {
			log.Println("INFO: Received API call for 'killmpd'.")
		}
		// This could probably be done more safely:
		cmd := exec.Command(strings.Fields(conf.MPD.Kill)[0], strings.Fields(conf.MPD.Kill)[1:]...)
		err := cmd.Run()
		if err != nil {
			log.Println("WARN: problem killing MPD: ", err)
		}
	})

	http.HandleFunc("/api/v1/killspinmpc", func(w http.ResponseWriter, r *http.Request) {
		if conf.Debug {
			log.Fatal("Exiting because of API call to 'killspinmpc'. Bye.")
		}
	})

	http.HandleFunc("/api/v1/listplaylists", func(w http.ResponseWriter, r *http.Request) {
		pls, err := (*conn).ListPlaylists()
		if err != nil {
			log.Println("WARN: failed to get playlists: ", err)
		}
		json.NewEncoder(w).Encode(pls)
	})

	http.HandleFunc("/api/v1/next", func(w http.ResponseWriter, r *http.Request) {
		err = (*conn).Next()
		if err != nil {
			log.Println("WARN: failed to skip to next track: ", err)
		}
	})

	http.HandleFunc("/api/v1/pause", func(w http.ResponseWriter, r *http.Request) {
		err = (*conn).Pause(true)
		if err != nil {
			log.Println("WARN: failed to pause: ", err)
		}
	})

	http.HandleFunc("/api/v1/play", func(w http.ResponseWriter, r *http.Request) {
		err = (*conn).Play(-1)
		if err != nil {
			log.Println("WARN: failed to play: ", err)
		}
	})

	http.HandleFunc("/api/v1/playlistload", func(w http.ResponseWriter, r *http.Request) {
		var err error
		decoder := json.NewDecoder(r.Body)
		var p = struct {
			Playlist string `json:"playlist"`
		}{}
		err = decoder.Decode(&p)
		if err != nil {
			log.Println(err)
		}
		defer r.Body.Close()
		err = (*conn).Clear()
		if err != nil {
			log.Println("WARN: failed to clear play queue: ", err)
		}
		err = (*conn).PlaylistLoad(p.Playlist, -1, -1)
		if err != nil {
			log.Println("WARN: failed to load playlist: ", err)
		}
		err = StartStop(*conn)
		if err != nil {
			log.Println(err)
		}
		if conf.Debug {
			log.Printf("INFO: loaded  playlist '%v'\n", p.Playlist)
		}
	})

	http.HandleFunc("/api/v1/playlists", func(w http.ResponseWriter, r *http.Request) {
		playlists, err := Playlists(*conn)
		if err != nil {
			log.Println(err)
		}
		json.NewEncoder(w).Encode(playlists)
	})

	http.HandleFunc("/api/v1/previous", func(w http.ResponseWriter, r *http.Request) {
		err = (*conn).Previous()
		if err != nil {
			log.Println("WARN: failed to skip to previous track: ", err)
		}
	})

	http.HandleFunc("/api/v1/randomtoggle", func(w http.ResponseWriter, r *http.Request) {
		s, err := (*conn).Status()
		if err != nil {
			log.Println(err)
		}
		switch s["random"] {
		case "1":
			(*conn).Random(false)
			s["random"] = "0"
		case "0":
			(*conn).Random(true)
			s["random"] = "1"
		}
		if conf.Debug {
			log.Println("INFO: random play mode changed to:", s["random"])
		}
		json.NewEncoder(w).Encode(s)
	})

	http.HandleFunc("/api/v1/reconnect", func(w http.ResponseWriter, r *http.Request) {
		err = Reconnect(conn, &conf)
		if err != nil {
			log.Println("WARN: failed to reconnect to MPD: ", err)
		}
	})

	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		s, err := (*conn).Status()
		if err != nil {
			log.Println("WARN: failed to get MPD status: ", err)
		}
		json.NewEncoder(w).Encode(s)
	})

	http.HandleFunc("/api/v1/stop", func(w http.ResponseWriter, r *http.Request) {
		err = (*conn).Stop()
		if err != nil {
			log.Println("WARN: failed to stop playback: ", err)
		}
	})

	http.HandleFunc("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		log.Println("TEST")
		decoder := json.NewDecoder(r.Body)
		var t = struct {
			Playlist string `json:"playlist"`
		}{}
		err := decoder.Decode(&t)
		if err != nil {
			log.Println(err)
		}
		defer r.Body.Close()
		log.Println(t.Playlist)
	})

	http.HandleFunc("/api/v1/updatempdatabase", func(w http.ResponseWriter, r *http.Request) {
		_, err = (*conn).Update("")
		if err != nil {
			log.Println("WARN: failed to update MPD database: ", err)
		}
	})

	log.Fatal(http.ListenAndServe(conf.Web.Address+":"+conf.Web.Port, nil))
}
