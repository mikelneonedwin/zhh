package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const ServiceName = "_zhh._tcp"

type BetaInfo struct {
	Hostname string
	IP       string
	Port     int
	Octet    int
	OS       string
}

func Register(port int, hostname string, octet int, osName string) (*mdns.Server, error) {
	txts := []string{
		"octet=" + strconv.Itoa(octet),
		"os=" + osName,
	}
	service, err := mdns.NewMDNSService(
		"zhh-"+hostname,
		ServiceName,
		"",
		"",
		port,
		[]net.IP{},
		txts,
	)
	if err != nil {
		return nil, fmt.Errorf("create mDNS service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("start mDNS server: %w", err)
	}
	return server, nil
}

func Discover(ctx context.Context, targetOctet int) ([]BetaInfo, error) {
	var (
		betas []BetaInfo
		mu    sync.Mutex
	)

	entries := make(chan *mdns.ServiceEntry, 50)

	go func() {
		for entry := range entries {
			beta := BetaInfo{
				Hostname: entry.Host,
				Port:     entry.Port,
			}
			if len(entry.AddrV4) > 0 {
				beta.IP = entry.AddrV4.String()
			} else if len(entry.AddrV6) > 0 {
				beta.IP = entry.AddrV6.String()
			}
			for _, txt := range entry.InfoFields {
				if idx := strings.Index(txt, "="); idx > 0 {
					key := txt[:idx]
					val := txt[idx+1:]
					switch key {
					case "octet":
						beta.Octet, _ = strconv.Atoi(val)
					case "os":
						beta.OS = val
					}
				}
			}
			if targetOctet > 0 && beta.Octet != targetOctet {
				continue
			}
			mu.Lock()
			betas = append(betas, beta)
			mu.Unlock()
		}
	}()

	params := &mdns.QueryParam{
		Service:   ServiceName,
		Domain:    "local",
		Timeout:   time.Second * 3,
		Entries:   entries,
	}

	if err := mdns.Query(params); err != nil {
		close(entries)
		return nil, fmt.Errorf("mDNS query: %w", err)
	}
	close(entries)

	mu.Lock()
	defer mu.Unlock()
	return betas, nil
}
