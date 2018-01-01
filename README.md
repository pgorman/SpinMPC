# SpinMPC #

SpinMPC is a simple web-based music player, a client for [mpd](https://www.musicpd.org/).
It's written in Go.

You may find SpinMPC too minimal for use as your everyday desktop music player.
That's OK.
SpinMPC's target use case is when you're hanging out with a bunch of friends, playing tunes from your media server.
With SpinMPC, any guest can use their own phone to check what's playing or skip to the next track without leaving the couch.

## Configuration ##

With no configuration file, SpinMPC starts with reasonable defaults.
Specify a configuration file with the `-c` flag.
The configuration file should be valid JSON.
Here's a sample configuration file:

	{
		"Debug": false,
		"MPD": {
			"Address": "127.0.0.1",
			"Port": "6600",
			"Password": ""
		},
		"Web": {
			"Address": "",
			"Port": "8870",
			"Password": "",
			"Root": "/var/www/spinmpc"
		}
	}

Options may be specified in the configuration file or with command-line flags.

Passwords specified on the command line are visible to other users on the system looking at the process list.
If this is a problem for you, put the password in a config file secured with appropriate permissions.
If no password is specified in the configuration file, SpinMPC allows anyone that can access its web interface (probably anyone on your LAN) to control the music.

## Troubleshooting ##

Enable debugging output with the `-d` flag.

## Links ##

- https://musicpd.org/
- https://musicpd.org/doc/protocol/
- https://wiki.archlinux.org/index.php/Music_Player_Daemon/Troubleshooting
- https://github.com/fhs/gompd
- https://paulgorman.org/technical/mpd.txt

## License (GPL) ##

SpinMPC copyright 2017 Paul Gorman, and licensed under the GNU General Public License.

https://www.gnu.org/licenses/gpl.html