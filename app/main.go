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
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	qrcode "github.com/skip2/go-qrcode"
	"google.golang.org/api/iterator"
)

// ─── Constantes ─────────────────────────────────────────────────────────────

const (
	bucketName   = "script-resize"
	inputPrefix  = "img/before/"
	outputPrefix = "img/after/"
	urlCacheTTL  = 30 * time.Minute
)

// ─── Types ──────────────────────────────────────────────────────────────────

// ImageEntry représente une image dans la galerie.
type ImageEntry struct {
	Filename  string    `json:"filename"`
	URL       string    `json:"url"`
	Uploader  string    `json:"uploader"`
	CreatedAt time.Time `json:"-"`
}

type cachedURL struct {
	url     string
	expires time.Time
}

// ─── Variables globales ─────────────────────────────────────────────────────

var templates *template.Template

// urlCache garde les signed URLs en cache pour éviter d'en regénérer
// à chaque requête (ce qui cause un clignotement côté navigateur).
var urlCache = struct {
	sync.RWMutex
	entries map[string]cachedURL
}{entries: make(map[string]cachedURL)}

// ─── Helpers ────────────────────────────────────────────────────────────────

// baseDir retourne le répertoire contenant templates/ et static/.
// En dev (air), c'est le working directory.
// En prod (scratch), c'est le répertoire de l'exécutable.
func baseDir() string {
	if _, err := os.Stat("templates"); err == nil {
		return "."
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// getCachedSignedURL retourne une signed URL depuis le cache, ou en génère une nouvelle.
func getCachedSignedURL(bucket *storage.BucketHandle, objectName string) (string, error) {
	urlCache.RLock()
	if cached, ok := urlCache.entries[objectName]; ok && time.Now().Before(cached.expires) {
		urlCache.RUnlock()
		return cached.url, nil
	}
	urlCache.RUnlock()

	signed, err := bucket.SignedURL(objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		return "", err
	}

	urlCache.Lock()
	urlCache.entries[objectName] = cachedURL{url: signed, expires: time.Now().Add(urlCacheTTL)}
	urlCache.Unlock()

	return signed, nil
}

// renderUploadError affiche la page upload avec un message d'erreur.
func renderUploadError(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	templates.ExecuteTemplate(w, "upload.html", map[string]string{"Error": msg})
}

// ─── GCS ────────────────────────────────────────────────────────────────────

// listAfterImages liste les images dans img/after/, triées par date d'ajout.
func listAfterImages(ctx context.Context, client *storage.Client) []ImageEntry {
	bucket := client.Bucket(bucketName)
	it := bucket.Objects(ctx, &storage.Query{Prefix: outputPrefix})
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

		signedURL, err := getCachedSignedURL(bucket, attrs.Name)
		if err != nil {
			log.Printf("SignedURL(%s): %v", attrs.Name, err)
			continue
		}

		images = append(images, ImageEntry{
			Filename:  filename,
			URL:       signedURL,
			Uploader:  uploader,
			CreatedAt: attrs.Created,
		})
	}

	sort.Slice(images, func(i, j int) bool {
		return images[i].CreatedAt.Before(images[j].CreatedAt)
	})

	return images
}

// ─── Handlers ───────────────────────────────────────────────────────────────

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

// ─── Point d'entrée ─────────────────────────────────────────────────────────

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
