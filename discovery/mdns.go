package discovery

import (
	"context"
	"fmt"
	"io"
	"log"
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

	var processWg sync.WaitGroup
	processWg.Add(1)

	go func() {
		defer processWg.Done()
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

	ifaces, err := net.Interfaces()
	if err != nil {
		close(entries)
		processWg.Wait()
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var wg sync.WaitGroup

	// Suppress standard log output to hide mDNS [INFO] logs
	oldLog := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldLog)

	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagMulticast) == 0 {
			continue
		}
		ifc := iface
		wg.Add(1)
		go func(i *net.Interface) {
			defer wg.Done()
			params := &mdns.QueryParam{
				Service:   ServiceName,
				Domain:    "local",
				Timeout:   time.Second * 3,
				Entries:   entries,
				Interface: i,
			}
			mdns.Query(params)
		}(&ifc)
	}

	wg.Wait()
	log.SetOutput(oldLog) // restore as soon as mdns.Query calls are done

	close(entries)
	processWg.Wait()

	mu.Lock()
	defer mu.Unlock()

	dedup := make([]BetaInfo, 0, len(betas))
	seen := make(map[string]bool)
	for _, b := range betas {
		key := fmt.Sprintf("%s:%s:%d", b.Hostname, b.IP, b.Port)
		if !seen[key] {
			seen[key] = true
			dedup = append(dedup, b)
		}
	}
	return dedup, nil
}
