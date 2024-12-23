package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var dsn string
	flag.Parse()
	if *real {
		dsn = "postgres://" + *dbUser + ":" + *dbPass + "@" + *dbAddr + "/" + *dbName
	} else {
		dsn = DsnFromEnv()
	}
	s := NewServer(dsn)
	defer s.DB.Close(context.Background())

	s.Intel.SavedIp4Addresses = s.GetIP4s()
	for v, ip4 := range s.Intel.SavedIp4Addresses {
		// c++
		// val := fmt.Sprintf("%v %v", c, ip4.Value)
		LocalCache.IPs = append(LocalCache.IPs, ip4.Value)
		fmt.Printf("%v ", v)
	}
	fmt.Println("\nloaded ips")
	ticker := time.NewTicker(20 * time.Second)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-ticker.C:
				if len(s.Intel.Ip4Addresses) == 0 {
					fmt.Println("No IP4s to save", len(s.Intel.SavedIp4Addresses))
					continue
				}
				fmt.Println("Saving IP4s")
				s.Memory.Lock()
				for octect, ips := range s.Intel.Ip4Addresses {
					s.BulkSaveIp4(octect, ips)
					delete(s.Intel.Ip4Addresses, octect)
				}
				s.Memory.Unlock()
			case <-sigs:
				ticker.Stop()
				os.Exit(0)
			case <-s.Stopch:
				ticker.Stop()
				os.Exit(0)
			}
		}
	}()
	log.Fatal(http.ListenAndServe(s.Addr, s.Gateway))
}
