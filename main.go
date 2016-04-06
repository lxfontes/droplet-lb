package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/digitalocean/godo"
	"github.com/miekg/dns"
	"golang.org/x/oauth2"
)

type DoApiConfig struct {
	AccessToken string
}

type DoApi struct {
	sync.RWMutex
	client         *godo.Client
	cachedDroplets map[string]godo.Droplet
}

func (cfg DoApiConfig) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: cfg.AccessToken,
	}
	return token, nil
}

func NewDoApi(cfg DoApiConfig) *DoApi {
	oauthClient := oauth2.NewClient(oauth2.NoContext, cfg)
	r := &DoApi{
		client: godo.NewClient(oauthClient),
	}

	return r
}

func (api *DoApi) Regions() ([]godo.Region, error) {
	regions, _, err := api.client.Regions.List(nil)
	return regions, err
}

func (api *DoApi) RefreshDroplets() error {
	droplets, err := api.Droplets()

	if err != nil {
		return err
	}

	newCache := make(map[string]godo.Droplet)
	for _, droplet := range droplets {
		if droplet.Status == "active" {
			newCache[droplet.Name] = droplet
		}
	}

	api.Lock()
	api.cachedDroplets = newCache
	api.Unlock()

	return nil
}

func (api *DoApi) Droplets() ([]godo.Droplet, error) {
	r := []godo.Droplet{}
	opt := &godo.ListOptions{}
	for {
		droplets, resp, err := api.client.Droplets.List(opt)
		if err != nil {
			return nil, err
		}

		for _, d := range droplets {
			r = append(r, d)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return r, nil
}

func (api *DoApi) FilterCachedDroplets(pattern string) []godo.Droplet {
	api.RLock()
	defer api.RUnlock()

	r := []godo.Droplet{}

	for name, droplet := range api.cachedDroplets {
		if strings.HasPrefix(name, pattern) {
			r = append(r, droplet)
		}
	}

	return r
}

func fillResponse(queryAddress string, droplets []godo.Droplet, msg *dns.Msg) {
	for _, droplet := range droplets {
		doIP, err := droplet.PublicIPv4()
		if err != nil {
			log.Println("Failed to get droplet ip", droplet.Name, err)
			return
		}

		a := net.ParseIP(doIP)
		rr := new(dns.A)
		rr.Hdr = dns.RR_Header{Name: queryAddress, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
		rr.A = a.To4()

		msg.Answer = append(msg.Answer, rr)
	}
}

func main() {
	var (
		token       = flag.String("token", os.Getenv("DO_KEY"), "Your Personal Key (Also reads from $DO_KEY)")
		domain      = flag.String("domain", "droplet-lb.", "Domain to respond queries for (note the dot at the end)")
		bindAddress = flag.String("bind", "127.0.0.1:8053", "Expose DNS interface at this address")
	)

	flag.Parse()
	if *token == "" {
		log.Println("Missing token")
		flag.PrintDefaults()
		os.Exit(2)
	}

	cfg := DoApiConfig{
		AccessToken: *token,
	}

	api := NewDoApi(cfg)
	err := api.RefreshDroplets()

	if err != nil {
		log.Println("Failed to contact DO api. Wrong api?")
		os.Exit(1)
	}

	dns.HandleFunc(*domain, func(w dns.ResponseWriter, r *dns.Msg) {
		if r.Question[0].Qtype != dns.TypeA {
			log.Println("Invalid query type", r.Question[0].Qtype, ". 'A' ipv4 only.")
			return
		}

		targetDots := strings.Split(r.Question[0].Name, ".")
		targetName := targetDots[0]

		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		droplets := api.FilterCachedDroplets(targetName)
		fillResponse(r.Question[0].Name, droplets, m)

		w.WriteMsg(m)
	})

	go func() {
		server := &dns.Server{Addr: *bindAddress, Net: "udp"}
		err = server.ListenAndServe()
		if err != nil {
			log.Println("Failed to bring up DNS server")
			os.Exit(1)
		}
	}()

	log.Println("Hit me up @", *bindAddress, "with domain", *domain)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-sig:
			log.Fatalf("Signal (%d) received, stopping\n", s)
		case <-time.After(1 * time.Minute):
			log.Println("Refreshing droplets")
			api.RefreshDroplets()
		}
	}

	os.Exit(0)
}
