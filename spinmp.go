// SpinMPC is a music player client for mpd.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

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
	}
}

func hdlOK(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK")
}

func main() {
	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Set up configuration from defaults, config file, and command-line flags.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	c := flag.String("c", "/etc/spinmp.conf", "Specify the full path to the configuration file.")
	debug = flag.Bool("d", false, "Turn on debugging messages.")
	mpdaddr := flag.String("mdpaddr", "", "Specify the address of the interface where MPD listens.")
	mpdport := flag.String("mdpport", "", "Specify the port on which MPD listens.")
	mpdpass := flag.String("mdppass", "", "Specify password required by MPD (if any).")
	webaddr := flag.String("webaddr", "", "Specify the address of the interface where SpinMPC serves its web interface.")
	webport := flag.String("webport", "", "Specify the port on which SpinMPC serves its web interface.")
	webpass := flag.String("webpass", "", "Password to require for access to SpinMPC's web interface.")
	flag.Parse()

	conf := Configuration{}
	conf.Debug = false
	conf.MPD.Address = "127.0.0.1"
	conf.MPD.Port = "6600"
	conf.MPD.Password = ""
	conf.Web.Address = ""
	conf.Web.Port = "8870"
	conf.Web.Password = ""

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
	}

	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Connect to MPD.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	conn, err := net.Dial("tcp", conf.MPD.Address+":"+conf.MPD.Port)
	if err != nil {
		log.Fatal("ERROR: can't connect to MPD: ", err)
	}
	defer conn.Close()
	conn.Write([]byte("status\n"))
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Println("WARN: can't get MPD status: ", err)
	}
	if conf.Debug {
		log.Println("INFO: testing connection to MPD:", string(status))
	}

	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
	*  Start serving web interface.
	* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */
	http.HandleFunc("/", hdlOK)
	log.Fatal(http.ListenAndServe(conf.Web.Address+":"+conf.Web.Port, nil))
}
