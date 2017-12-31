// SpinMPC is a music player client for mpd.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
)

var err error
var debug *bool

type Configuration struct {
	Debug bool
	MPD   struct {
		Address  string
		Port     string
		Password string
	}
	Web struct {
		Address  string
		Port     string
		Password string
		Root     string
		Search   string
	}
}

type Status struct {
	conn *mpd.Client
}

func (h *Status) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "SpinMPC web interface: OK\n")
	status, err := h.conn.Status()
	if err != nil {
		io.WriteString(w, strings.Join([]string{"Can't get MPD status: ", err.Error(), "\n"}, ""))
	}
	for k, v := range status {
		io.WriteString(w, strings.Join([]string{"MPD status: ", k, ": ", v, "\n"}, ""))
	}
	stats, err := h.conn.Stats()
	if err != nil {
		io.WriteString(w, strings.Join([]string{"Can't get MPD stats: ", err.Error(), "\n"}, ""))
	}
	for k, v := range stats {
		io.WriteString(w, strings.Join([]string{"MPD stat: ", k, ": ", v, "\n"}, ""))
	}
}

// fillPlaylist populates the default playlist with all files in the database.
func fillPlaylist(conn *mpd.Client) {
	err = conn.Clear()
	if err != nil {
		log.Println("WARN: failed to clear playlist: ", err)
	}
	songs, err := conn.GetFiles()
	if err != nil {
		log.Println(err)
	}
	for _, s := range songs {
		err = conn.Add(s)
		if err != nil {
			log.Println("WARN: can't add file to playlist: ", err)
		}
	}
}

// keepAlive keeps our connection to MPD open.
func keepAlive(conn *mpd.Client) {
	for {
		err = conn.Ping()
		if err != nil {
			log.Println("WARN: can't pring MPD: ", err)
		}
		time.Sleep(time.Second * 5)
	}
}

// searchURL constructs a URL to web search a song.
func searchURL(conf *Configuration, song map[string]string) string {
	q := url.QueryEscape(strings.Join([]string{"\"", song["Artist"], "\" \"", song["Title"], "\" \"", song["Album"], "\""}, ""))
	return strings.Join([]string{conf.Web.Search, q}, "")
}

func main() {
	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Set up configuration from defaults, config file, and command-line flags.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	c := flag.String("c", "/etc/spinmpc.conf", "Specify the full path to the configuration file.")
	debug = flag.Bool("d", false, "Turn on debugging messages.")
	mpdaddr := flag.String("mdpaddr", "", "Specify the address of the interface where MPD listens.")
	mpdport := flag.String("mdpport", "", "Specify the port on which MPD listens.")
	mpdpass := flag.String("mdppass", "", "Specify password required by MPD (if any).")
	webaddr := flag.String("webaddr", "", "Specify the address of the interface where SpinMPC serves its web interface.")
	webport := flag.String("webport", "", "Specify the port on which SpinMPC serves its web interface.")
	webpass := flag.String("webpass", "", "Password to require for access to SpinMPC's web interface.")
	webroot := flag.String("webroot", "", "Directory from which to serve SpinMPC's web documents.")
	websearch := flag.String("websearch", "", "Set base URL for web searches.")
	flag.Parse()

	conf := Configuration{}
	conf.Debug = false
	conf.MPD.Address = "127.0.0.1"
	conf.MPD.Port = "6600"
	conf.MPD.Password = ""
	conf.Web.Address = ""
	conf.Web.Port = "8870"
	conf.Web.Password = ""
	conf.Web.Root = "./"
	conf.Web.Search = "https://duckduckgo.com/?q="

	f, err := os.Open(*c)
	if err != nil {
		log.Println("WARN: can't open config file: ", err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&conf)
	if err != nil {
		log.Println("WARN: can't decode config JSON: ", err)
	}

	if *debug {
		conf.Debug = *debug
	}
	if *mpdaddr != "" {
		conf.MPD.Address = *mpdaddr
	}
	if *mpdport != "" {
		conf.MPD.Port = *mpdport
	}
	if *mpdpass != "" {
		conf.MPD.Password = *mpdpass
	}
	if *webaddr != "" {
		conf.Web.Address = *webaddr
	}
	if *webport != "" {
		conf.Web.Port = *webport
	}
	if *webpass != "" {
		conf.Web.Password = *webpass
	}
	if *webroot != "" {
		conf.Web.Root = *webroot
	}
	if *websearch != "" {
		conf.Web.Search = *websearch
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

	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Connect to MPD.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	conn, err := mpd.Dial("tcp", conf.MPD.Address+":"+conf.MPD.Port)
	if err != nil {
		log.Fatal("ERROR: can't connect to MPD: ", err)
	}
	defer conn.Close()
	status, err := conn.Status()
	if err != nil {
		log.Println("WARN: can't get MPD status: ", err)
	}
	if conf.Debug {
		log.Println("INFO: checking initial MPD status...")
		for k, v := range status {
			log.Println("INFO: MPD status:", k, v)
		}
	}
	stats, err := conn.Stats()
	if err != nil {
		log.Println("WARN: can't get MPD stats: ", err)
	}
	if conf.Debug {
		log.Println("INFO: fetching MPD stats...")
		for k, v := range stats {
			log.Println("INFO: MPD stat:", k, v)
		}
	}
	go keepAlive(conn)
	fillPlaylist(conn)
	// Start and immiately stop, so there's a song populated in "status".
	err = conn.Play(-1)
	if err != nil {
		log.Println("WARN: failed to stop playback: ", err)
	}
	err = conn.Stop()
	if err != nil {
		log.Println("WARN: failed to stop playback: ", err)
	}

	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Start serving web interface.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(conf.Web.Root, "index.html"))
	})

	http.HandleFunc("/api/v1/currentsong", func(w http.ResponseWriter, r *http.Request) {
		s, err := conn.CurrentSong()
		if err != nil {
			log.Println("WARN: failed to get current song info: ", err)
		}
		s["SearchURL"] = searchURL(&conf, s)
		json.NewEncoder(w).Encode(s)
	})

	http.HandleFunc("/api/v1/next", func(w http.ResponseWriter, r *http.Request) {
		err = conn.Next()
		if err != nil {
			log.Println("WARN: failed to skip to next track: ", err)
		}
	})

	http.HandleFunc("/api/v1/pause", func(w http.ResponseWriter, r *http.Request) {
		err = conn.Pause(true)
		if err != nil {
			log.Println("WARN: failed to pause: ", err)
		}
	})

	http.HandleFunc("/api/v1/play", func(w http.ResponseWriter, r *http.Request) {
		err = conn.Play(-1)
		if err != nil {
			log.Println("WARN: failed to play: ", err)
		}
	})

	http.HandleFunc("/api/v1/previous", func(w http.ResponseWriter, r *http.Request) {
		err = conn.Previous()
		if err != nil {
			log.Println("WARN: failed to skip to previous track: ", err)
		}
	})

	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		s, err := conn.Status()
		if err != nil {
			log.Println("WARN: failed to get MPD status: ", err)
		}
		json.NewEncoder(w).Encode(s)
	})

	http.HandleFunc("/api/v1/stop", func(w http.ResponseWriter, r *http.Request) {
		err = conn.Stop()
		if err != nil {
			log.Println("WARN: failed to stop playback: ", err)
		}
	})

	http.Handle("/status", &Status{conn})
	log.Fatal(http.ListenAndServe(conf.Web.Address+":"+conf.Web.Port, nil))
}
