# deepfry
a tool for determining if an IP or URL passed to it has been passed to it before. IPs are simply stored, url requests are counted and stored / updated.

## routes

- "/view", view dashboard
- "/ip4", add IP /  get novel status, `for i in $(cat ~/FILENAME); do curl -X POST HOST:8080/ip4 -d "{\"value\": \"$i\"}"; done`
- "/urls", add url / get novel status, `for i in $(cat ~/FILENAME); do curl -X POST HOST:8080/urls -d "{\"value\": \"$i\"}"; done`
- "/stats", get url stats

### to see more routes view
func NewServer(dsn string) *Server

# thanks to *in no order*
intel one mono font
postgresql database
the never ending urge to build that thing i need for that one task