package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	defer func() {
		fmt.Println("Closing database connection pool...")
		s.DB.Close()
	}()

	s.Intel.SavedIp4Addresses = s.GetIP4s()
	s.Intel.Stats = s.GetStats()
	for _, ip4 := range s.Intel.SavedIp4Addresses {
		LocalCache.IPs = append(LocalCache.IPs, ip4.Value)
	}
	fmt.Println("\nloaded", len(s.Intel.SavedIp4Addresses), "ip4s")
	ticker := time.NewTicker(20 * time.Second)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("Processing IPs in octets...")
				s.TestConnection()
				s.Memory.Lock()
				const maxIPsPerBatchCall = 500
				wg := &sync.WaitGroup{}

				octetsToProcess := make([]string, 0, len(s.Intel.Ip4Addresses))

				for octet := range s.Intel.Ip4Addresses {
					octetsToProcess = append(octetsToProcess, octet)
				}
				fmt.Printf("Processing %d octets for IPs (%v)\n", len(octetsToProcess), len(s.Intel.Ip4Addresses))

				for _, octet := range octetsToProcess {
					ipsInOctet := s.Intel.Ip4Addresses[octet]
					if len(ipsInOctet) == 0 {
						continue
					}

					for len(ipsInOctet) > 0 {
						chunkSize := maxIPsPerBatchCall
						if len(ipsInOctet) < maxIPsPerBatchCall {
							chunkSize = len(ipsInOctet)
						}

						batchToSave := ipsInOctet[:chunkSize]

						fmt.Printf("Saving %d IP(s) for octet %s (total pending for octet: %d)\n", len(batchToSave), octet, len(ipsInOctet))
						s.Stats["BulkSaveIp4_calls"]++

						batchCopy := make([]Ip4, len(batchToSave))
						copy(batchCopy, batchToSave)

						wg.Add(1)
						go func(o string, b []Ip4) {
							defer wg.Done()
							s.BulkSaveIp4(o, b)
						}(octet, batchCopy)

						ipsInOctet = ipsInOctet[chunkSize:]
						s.Intel.Ip4Addresses[octet] = ipsInOctet

						if len(s.Intel.Ip4Addresses[octet]) == 0 {
							fmt.Printf("Finished processing all IPs for octet %s.\n", octet)
							break
						}
					}
				}
				s.Intel.SetRuntimeStats(s.Stats)
				s.Memory.Unlock()
				wg.Wait()
				go s.SaveStats()
			case <-sigs:
				ticker.Stop()
				os.Exit(0)
			case <-s.Stopch:
				ticker.Stop()
				os.Exit(0)
			}
		}
	}()
	fmt.Println("Server is starting...")
	log.Fatal(http.ListenAndServe(s.Addr, s.Gateway))
}
