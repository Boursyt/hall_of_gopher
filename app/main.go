package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	qrcode "github.com/skip2/go-qrcode"
	"google.golang.org/api/iterator"
)

const (
	bucketName   = "script-resize"
	inputPrefix  = "img/before/"
	outputPrefix = "img/after/"
)

var templates *template.Template

// ImageEntry représente une image dans la galerie.
type ImageEntry struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Uploader string `json:"uploader"`
}

// baseDir retourne le répertoire contenant templates/ et static/.
// En prod (scratch), c'est le répertoire de l'exécutable.
// En dev (air), c'est le working directory.
func baseDir() string {
	// Si templates/ existe dans le cwd, on l'utilise (dev)
	if _, err := os.Stat("templates"); err == nil {
		return "."
	}
	// Sinon on utilise le répertoire de l'exécutable (prod)
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func main() {
	base := baseDir()
	templates = template.Must(template.ParseGlob(filepath.Join(base, "templates", "*.html")))

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(base, "static")))))

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/home", handleHome)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/api/images", handleAPIImages)
	http.HandleFunc("/qrcode", handleQRCode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// handleIndex redirige vers /home.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/home", http.StatusFound)
}

// handleHome affiche la page d'accueil avec le slider.
func handleHome(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "home.html", nil)
}

// handleUpload affiche le formulaire (GET) ou traite l'upload (POST).
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		templates.ExecuteTemplate(w, "upload.html", nil)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		renderUploadError(w, "Fichier trop volumineux (max 10MB).")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	file, header, err := r.FormFile("image")
	if err != nil || name == "" {
		renderUploadError(w, "Le nom et l'image sont obligatoires.")
		return
	}
	defer file.Close()

	gcsFilename := name + "_" + header.Filename

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		log.Printf("storage.NewClient: %v", err)
		return
	}
	defer client.Close()

	gcsWriter := client.Bucket(bucketName).Object(inputPrefix + gcsFilename).NewWriter(ctx)
	gcsWriter.ContentType = header.Header.Get("Content-Type")

	if _, err := io.Copy(gcsWriter, file); err != nil {
		gcsWriter.Close()
		http.Error(w, "Erreur upload", http.StatusInternalServerError)
		log.Printf("io.Copy: %v", err)
		return
	}

	if err := gcsWriter.Close(); err != nil {
		http.Error(w, "Erreur upload", http.StatusInternalServerError)
		log.Printf("writer.Close: %v", err)
		return
	}

	http.Redirect(w, r, "/home", http.StatusFound)
}

// handleAPIImages retourne la liste des images en JSON.
func handleAPIImages(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		log.Printf("storage.NewClient: %v", err)
		return
	}
	defer client.Close()

	images := listAfterImages(ctx, client)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

// handleQRCode génère un QR code PNG pointant vers /home.
func handleQRCode(w http.ResponseWriter, r *http.Request) {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}

	target := fmt.Sprintf("%s://%s/home", scheme, r.Host)

	qr, err := qrcode.New(target, qrcode.Medium)
	if err != nil {
		http.Error(w, "Erreur QR", http.StatusInternalServerError)
		log.Printf("qrcode.New: %v", err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, qr.Image(256))
}

// listAfterImages liste les images dans img/after/ et extrait le nom de l'uploadeur.
func listAfterImages(ctx context.Context, client *storage.Client) []ImageEntry {
	it := client.Bucket(bucketName).Objects(ctx, &storage.Query{Prefix: outputPrefix})
	var images []ImageEntry

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("iterator.Next: %v", err)
			break
		}

		filename := strings.TrimPrefix(attrs.Name, outputPrefix)
		if filename == "" {
			continue
		}

		uploader := "Inconnu"
		if parts := strings.SplitN(filename, "_", 2); len(parts) > 1 {
			uploader = parts[0]
		}

		images = append(images, ImageEntry{
			Filename: filename,
			URL:      fmt.Sprintf("https://storage.googleapis.com/%s/%s%s", bucketName, outputPrefix, filename),
			Uploader: uploader,
		})
	}

	return images
}

// renderUploadError affiche la page upload avec un message d'erreur.
func renderUploadError(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	templates.ExecuteTemplate(w, "upload.html", map[string]string{"Error": msg})
}
