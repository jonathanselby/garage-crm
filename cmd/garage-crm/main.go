// Garage CRM is a single-user, offline customer relationship manager.
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/app"
	"github.com/jonathanselby/garage-crm/internal/portable"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"github.com/jonathanselby/garage-crm/internal/web"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "0.1.1-dev"

func rootDir() string {
	if p := os.Getenv("XDG_DATA_HOME"); p != "" {
		return filepath.Join(p, "garage-crm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "garage-crm")
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "garage-crm:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	switch args[0] {
	case "version", "--version":
		fmt.Println("garage-crm", version)
		return nil
	case "help", "--help", "-h":
		fmt.Println("garage-crm serve [-data DIR] [-port 43178] [-no-browser]\ngarage-crm export [-data DIR] -output FILE.zip\ngarage-crm import -input FILE.zip [-data NEW_DIR]\ngarage-crm inspect -input FILE.zip\ngarage-crm backup [-data DIR] -output FILE.db\ngarage-crm version")
		return nil
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		root := fs.String("data", rootDir(), "private CRM data directory")
		port := fs.Int("port", 43178, "loopback TCP port (0 means choose)")
		noBrowser := fs.Bool("no-browser", false, "do not launch browser")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *port < 0 || *port > 65535 {
			return errors.New("invalid port")
		}
		return serve(*root, *port, *noBrowser)
	case "export", "backup":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		root := fs.String("data", rootDir(), "data directory")
		out := fs.String("output", "", "output filename; must not exist")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *out == "" {
			return errors.New("-output is required")
		}
		db, err := openRoot(*root)
		if err != nil {
			return err
		}
		defer db.Close()
		if args[0] == "export" {
			err = portable.Export(db, *out)
		} else {
			err = db.Backup(*out)
		}
		if err != nil {
			return err
		}
		fmt.Println("Created", *out)
		return nil
	case "import":
		fs := flag.NewFlagSet("import", flag.ContinueOnError)
		root := fs.String("data", rootDir(), "new private data directory; must be empty")
		in := fs.String("input", "", "portable ZIP archive")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *in == "" {
			return errors.New("-input is required")
		}
		if err := os.MkdirAll(*root, 0700); err != nil {
			return err
		}
		_, err := os.Stat(filepath.Join(*root, "crm.db"))
		if err == nil {
			return errors.New("existing CRM data found; import into a clean location")
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := portable.ImportIntoEmpty(*in, filepath.Join(*root, "crm.db")); err != nil {
			return err
		}
		fmt.Println("Imported into", *root)
		return nil
	case "inspect":
		fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
		in := fs.String("input", "", "archive")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		m, err := portable.Inspect(*in)
		if err != nil {
			return err
		}
		fmt.Printf("%s; schema %d; %d tables; exported %s\n", m.Format, m.Schema, len(m.Files), m.ExportedAt)
		for _, f := range m.Files {
			fmt.Printf("%s: %d rows, %d bytes\n", f.Name, f.Count, f.Bytes)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q; use help", args[0])
	}
}
func openRoot(root string) (*sqlite.DB, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(filepath.Join(root, "crm.db"))
	if err != nil {
		return nil, err
	}
	if err = app.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func serve(root string, port int, noBrowser bool) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".process.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another Garage CRM instance is already using this data directory")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	db, err := openRoot(root)
	if err != nil {
		return err
	}
	defer db.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	defer listener.Close()
	addr := listener.Addr().String()
	server := web.New(&app.Service{DB: db}, addr)
	bootstrapFile, err := server.BootstrapFile(root)
	if err != nil {
		return err
	}
	defer os.Remove(bootstrapFile)
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 3 * time.Minute, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 8192}
	signalCtx := make(chan os.Signal, 1)
	signal.Notify(signalCtx, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCtx)
	stopped := make(chan struct{})
	go func() { <-signalCtx; httpServer.Close(); close(stopped) }()
	fmt.Println("Garage CRM", version, "— local only / offline")
	fmt.Println("Local address:", "http://"+addr)
	fmt.Println("Private browser starter:", bootstrapFile)
	fmt.Println("Data directory:", root)
	if !noBrowser {
		if path, err := exec.LookPath("xdg-open"); err == nil {
			cmd := exec.Command(path, bootstrapFile)
			if err := cmd.Start(); err == nil {
				go cmd.Wait()
			} else {
				log.Printf("desktop launcher unavailable: %v", err)
			}
		}
	}
	err = httpServer.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	select {
	case <-stopped:
	default:
	}
	return nil
}

var _ = strings.TrimSpace
