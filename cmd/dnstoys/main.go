package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/knadh/dns.toys/internal/geo"
	"github.com/knadh/dns.toys/internal/services/aerial"
	"github.com/knadh/dns.toys/internal/services/base"
	"github.com/knadh/dns.toys/internal/services/cidr"
	"github.com/knadh/dns.toys/internal/services/coin"
	"github.com/knadh/dns.toys/internal/services/dice"
	"github.com/knadh/dns.toys/internal/services/dict"
	"github.com/knadh/dns.toys/internal/services/epoch"
	"github.com/knadh/dns.toys/internal/services/fx"
	"github.com/knadh/dns.toys/internal/services/num2words"
	"github.com/knadh/dns.toys/internal/services/random"
	"github.com/knadh/dns.toys/internal/services/timezones"
	"github.com/knadh/dns.toys/internal/services/units"
	"github.com/knadh/dns.toys/internal/services/weather"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/miekg/dns"
	flag "github.com/spf13/pflag"
)

var (
	lo = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	ko = koanf.New(".")

	// Version of the build injected at build time.
	buildString = "unknown"
)

// Not all platforms have syscall.SIGUNUSED so use Golang's default definition here
const SIGUNUSED = syscall.Signal(0x1f)

func initConfig() {
	// Register --help handler.
	f := flag.NewFlagSet("config", flag.ContinueOnError)
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}
	f.StringSlice("config", []string{"config.toml"}, "path to one or more TOML config files to load in order")
	f.Bool("version", false, "show build version")
	f.Parse(os.Args[1:])

	// Display version.
	if ok, _ := f.GetBool("version"); ok {
		fmt.Println(buildString)
		os.Exit(0)
	}

	// Read the config files.
	cFiles, _ := f.GetStringSlice("config")
	for _, f := range cFiles {
		lo.Printf("reading config: %s", f)
		if err := ko.Load(file.Provider(f), toml.Parser()); err != nil {
			lo.Printf("error reading config: %v", err)
		}
	}

	ko.Load(posflag.Provider(f, ".", ko), nil)
}

func saveSnapshot(h *handlers) {
	interruptSignal := make(chan os.Signal)
	signal.Notify(interruptSignal,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGINT,
		SIGUNUSED, // SIGUNUSED, can be used to avoid shutting down the app.
	)

	// On receiving an OS signal, iterate through services and
	// dump their snapshots to the disk if available.
	for {
		select {
		case i := <-interruptSignal:
			lo.Printf("received SIGNAL: `%s`", i.String())

			for name, s := range h.services {
				if !ko.Bool(name+".enabled") || !ko.Bool(name+".snapshot_enabled") {
					continue
				}

				b, err := s.Dump()
				if err != nil {
					lo.Printf("error generating %s snapshot: %v", name, err)
				}

				if b == nil {
					continue
				}

				filePath := ko.MustString(name + ".snapshot_file")
				lo.Printf("saving %s snapshot to %s", name, filePath)
				if err := ioutil.WriteFile(filePath, b, 0644); err != nil {
					lo.Printf("error writing %s snapshot: %v", name, err)
				}
			}

			if i != SIGUNUSED {
				os.Exit(0)
			}
		}
	}
}

func loadSnapshot(service string) []byte {
	if !ko.Bool(service + ".snapshot_enabled") {
		return nil
	}

	filePath := ko.MustString(service + ".snapshot_file")

	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		lo.Printf("error reading snapshot file %s: %v", filePath, err)
		return nil
	}

	return b
}

func main() {
	initConfig()

	var (
		h = &handlers{
			services: make(map[string]Service),
			domain:   ko.MustString("server.domain"),
		}
		ge  *geo.Geo
		mux = dns.NewServeMux()

		help = [][]string{}
	)

	// Timezone service.
	if ko.Bool("timezones.enabled") || ko.Bool("weather.enabled") {
		fPath := ko.MustString("timezones.geo_filepath")
		lo.Printf("reading geo locations from %s", fPath)

		g, err := geo.New(fPath)
		if err != nil {
			lo.Fatalf("error loading geo locations: %v", err)
		}
		ge = g

		lo.Printf("%d geo location names loaded", g.Count())
	}

	// Timezone service.
	if ko.Bool("timezones.enabled") {
		tz := timezones.New(timezones.Opt{}, ge)
		h.register("time.zldns", tz, mux)

		help = append(help, []string{"get time for a city", "kdig budapest.time.zldns +tls @%s"})
	}

	// FX currency conversion.
	if ko.Bool("fx.enabled") {
		f := fx.New(fx.Opt{
			RefreshInterval: ko.MustDuration("fx.refresh_interval"),
		})

		// Load snapshot?
		if b := loadSnapshot("fx"); b != nil {
			if err := f.Load(b); err != nil {
				lo.Printf("error reading fx snapshot: %v", err)
			}
		}

		h.register("exchange.zldns", f, mux)

		help = append(help, []string{"convert currency rates", "kdig 100EUR-HUF.exchange.zldns +tls @%s"})
	}

	// IP echo.
	if ko.Bool("ip.enabled") {
		mux.HandleFunc("ip.zldns", h.handleEchoIP)

		help = append(help, []string{"get your host's requesting IP.", "kdig ip +tls @%s"})
	}

	// Weather.
	if ko.Bool("weather.enabled") {
		w := weather.New(weather.Opt{
			MaxEntries:       ko.MustInt("weather.max_entries"),
			ForecastInterval: ko.MustDuration("weather.forecast_interval"),
			CacheTTL:         ko.MustDuration("weather.cache_ttl"),
			ReqTimeout:       time.Second * 3,
			UserAgent:        ko.MustString("server.domain"),
		}, ge)

		// Load snapshot?
		if b := loadSnapshot("weather"); b != nil {
			if err := w.Load(b); err != nil {
				lo.Printf("error reading weather snapshot: %v", err)
			}
		}

		h.register("weather.zldns", w, mux)

		help = append(help, []string{"weather info", "kdig budapest.weather.zldns +tls @%s"})
	}

	// Units.
	if ko.Bool("units.enabled") {
		u, err := units.New()
		if err != nil {
			lo.Fatalf("error initializing units service: %v", err)
		}
		h.register("unit.zldns", u, mux)

		help = append(help, []string{"convert between units.", "kdig 1GB-MB.unit.zldns +tls @%s"})
	}

	// Numbers to words.
	if ko.Bool("num2words.enabled") {
		n := num2words.New()
		h.register("words.zldns", n, mux)

		help = append(help, []string{"convert numbers to words.", "kdig 123456.words.zldns +tls @%s"})
	}

	// CIDR.
	if ko.Bool("cidr.enabled") {
		n := cidr.New()
		h.register("cidr.zldns", n, mux)

		help = append(help, []string{"convert cidr to ip range.", "kdig 172.16.0.0/16.cidr.zldns @%s"})
	}

	// PI.
	if ko.Bool("pi.enabled") {
		mux.HandleFunc("pi.zldns.", h.handlePi)

		help = append(help, []string{"return digits of Pi as TXT or A or AAAA record.", "kdig pi +tls @%s"})
	}

	// Base
	if ko.Bool("base.enabled") {
		n := base.New()
		h.register("base.zldns", n, mux)

		help = append(help, []string{"convert numbers from one base to another", "kdig 100dec-hex.base.zldns @%s"})
	}

	// Dictionary.
	if ko.Bool("dict.enabled") {
		d := dict.New(dict.Opt{
			WordNetPath: ko.MustString("dict.wordnet_path"),
			MaxResults:  ko.MustInt("dict.max_results"),
		})
		h.register("dict.zldns", d, mux)

		help = append(help, []string{"get the definition of an English word, powered by WordNet(R).", "kdig fun.dict +tls @%s"})
	}

	// Rolling dice
	if ko.Bool("dice.enabled") {
		n := dice.New()
		h.register("dice.zldns", n, mux)

		help = append(help, []string{"roll dice", "kdig 1d6.dice.zldns @%s"})
	}

	// Random number generator.
	if ko.Bool("rand.enabled") {
		// seed the RNG:
		rand.Seed(time.Now().Unix())

		n := random.New()
		h.register("rand.zldns", n, mux)

		help = append(help, []string{"generate random numbers", "kdig 1-100.rand.zldns +tls @%s"})
	}

	// Coin toss.
	if ko.Bool("coin.enabled") {
		n := coin.New()
		h.register("coin.zldns", n, mux)

		help = append(help, []string{"toss coin", "kdig 2.coin.zldns +tls @%s"})
	}

	// Epoch / Unix timestamp conversion.
	if ko.Bool("epoch.enabled") {
		n := epoch.New(ko.Bool("epoch.send_local_time"))
		h.register("epoch.zldns", n, mux)

		help = append(help, []string{"convert epoch / UNIX time to human readable time.", "kdig 784783800.epoch.zldns +tls @%s"})
	}

	// Aerial Distance between Lat,Lng
	if ko.Bool("aerial.enabled") {
		a := aerial.New()
		h.register("aerial.zldns", a, mux)

		help = append(help, []string{"get aerial distance between lat lng pair", "kdig A12.9352,77.6245/12.9698,77.7500.aerial.zldns +tls @%s"})
	}

	// Prepare the static help response for the `help` query.
	for _, l := range help {
		r, err := dns.NewRR(fmt.Sprintf("help. 1 TXT \"%s\" \"%s\"", l[0], fmt.Sprintf(l[1], h.domain)))
		if err != nil {
			lo.Fatalf("error preparing: %v", err)
		}

		h.help = append(h.help, r)
	}

	mux.HandleFunc("help.zldns.", h.handleHelp)
	mux.HandleFunc(".", (h.handleDefault))

	// Start the snapshot listener.
	go saveSnapshot(h)

	// Start the server.
	server := &dns.Server{
		Addr:    ko.MustString("server.address"),
		Net:     "udp",
		Handler: mux,
	}
	lo.Println("listening on ", ko.String("server.address"))
	if err := server.ListenAndServe(); err != nil {
		lo.Fatalf("error starting server: %v", err)
	}
	defer server.Shutdown()
}
