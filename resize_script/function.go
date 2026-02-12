// Package function contient la Cloud Function de resize d'images.
// Déclenchée par un upload dans img/before/, elle crop l'image
// au centre en 800x600 et la sauvegarde dans img/after/.
package function

import (
	"context"
	"fmt"
	"image"
	"log"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/disintegration/imaging"
)

const (
	targetWidth  = 800
	targetHeight = 600
	jpegQuality  = 85
	inputPrefix  = "img/before/"
	outputPrefix = "img/after/"
)

// GCSEvent représente les données d'un événement Cloud Storage.
type GCSEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

func init() {
	functions.CloudEvent("ResizeImage", ResizeImage)
}

// ResizeImage est le point d'entrée de la Cloud Function.
// Elle télécharge l'image depuis img/before/, la crop, et l'enregistre dans img/after/.
func ResizeImage(ctx context.Context, cloudEvent event.Event) error {
	eventData, err := parseEvent(cloudEvent)
	if err != nil {
		return err
	}

	if !shouldProcess(eventData.Name) {
		return nil
	}

	totalStart := time.Now()

	bucket, cleanup, err := newBucket(ctx, eventData.Bucket)
	if err != nil {
		return err
	}
	defer cleanup()

	dlStart := time.Now()
	originalImage, err := downloadImage(ctx, bucket, eventData.Name)
	if err != nil {
		return err
	}
	dlDuration := time.Since(dlStart)

	cropStart := time.Now()
	croppedImage := cropToTarget(originalImage)
	cropDuration := time.Since(cropStart)

	outputPath := buildOutputPath(eventData.Name)
	upStart := time.Now()
	if err := uploadImage(ctx, bucket, outputPath, croppedImage); err != nil {
		return err
	}
	upDuration := time.Since(upStart)

	log.Printf("%s -> %s (%dx%d) | download: %s | crop: %s | upload: %s | total: %s",
		eventData.Name, outputPath, targetWidth, targetHeight,
		dlDuration, cropDuration, upDuration, time.Since(totalStart))
	return nil
}

// parseEvent extrait les données GCS depuis le CloudEvent.
func parseEvent(cloudEvent event.Event) (GCSEvent, error) {
	var data GCSEvent
	if err := cloudEvent.DataAs(&data); err != nil {
		return GCSEvent{}, fmt.Errorf("parse event: %w", err)
	}
	return data, nil
}

// shouldProcess vérifie que le fichier est une image dans le dossier d'entrée.
func shouldProcess(objectName string) bool {
	if !strings.HasPrefix(objectName, inputPrefix) {
		return false
	}
	return isImage(objectName)
}

// isImage vérifie l'extension du fichier.
func isImage(fileName string) bool {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	}
	return false
}

// newBucket crée un client Storage et retourne le bucket handle + une fonction de cleanup.
func newBucket(ctx context.Context, bucketName string) (*storage.BucketHandle, func(), error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("storage client: %w", err)
	}
	return client.Bucket(bucketName), func() { client.Close() }, nil
}

// downloadImage télécharge et décode une image depuis le bucket.
func downloadImage(ctx context.Context, bucket *storage.BucketHandle, objectPath string) (image.Image, error) {
	reader, err := bucket.Object(objectPath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", objectPath, err)
	}
	defer reader.Close()

	decodedImage, err := imaging.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", objectPath, err)
	}
	return decodedImage, nil
}

// cropToTarget redimensionne et crop l'image au centre pour atteindre la taille cible.
func cropToTarget(sourceImage image.Image) image.Image {
	return imaging.Fill(sourceImage, targetWidth, targetHeight, imaging.Center, imaging.Lanczos)
}

// uploadImage encode l'image en JPEG et l'upload dans le bucket.
func uploadImage(ctx context.Context, bucket *storage.BucketHandle, objectPath string, img image.Image) error {
	writer := bucket.Object(objectPath).NewWriter(ctx)
	writer.ContentType = "image/jpeg"

	if err := imaging.Encode(writer, img, imaging.JPEG, imaging.JPEGQuality(jpegQuality)); err != nil {
		writer.Close()
		return fmt.Errorf("encode %s: %w", objectPath, err)
	}
	return writer.Close()
}

// buildOutputPath transforme un chemin img/before/photo.jpg en img/after/photo.jpg.
func buildOutputPath(inputPath string) string {
	fileName := strings.TrimPrefix(inputPath, inputPrefix)
	return outputPrefix + fileName
}
