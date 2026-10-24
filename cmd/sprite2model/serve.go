package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/Coldensjo/sprite2model/spritemodel"
	"github.com/Coldensjo/sprite2model/web"
)

// serve runs the converter's page on this machine and opens it in the browser. The page is the one
// the GitHub page hosts; here it converts through /api/convert, in Go, instead of WebAssembly.
func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:0", "where to listen (port 0: any free one)")
	noOpen := fs.Bool("no-open", false, "do not open the browser")
	_ = fs.Parse(args)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(web.Files))
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": spritemodel.Version})
	})
	// POST the sprite's bytes; the query holds name and options (JSON, as spritemodel.Options).
	mux.HandleFunc("/api/convert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST a sprite", http.StatusMethodNotAllowed)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var opt spritemodel.Options
		if o := r.URL.Query().Get("options"); o != "" {
			if err := json.Unmarshal([]byte(o), &opt); err != nil {
				http.Error(w, "options: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		res, err := spritemodel.Run(r.URL.Query().Get("name"), data, opt)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(res)
	})

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("sprite2model %s on %s (Ctrl+C to stop)\n", spritemodel.Version, url)
	if !*noOpen {
		openBrowser(url)
	}
	log.Fatal(http.Serve(ln, mux))
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "open the page yourself:", url)
	}
}
