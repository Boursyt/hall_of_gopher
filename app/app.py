import io

from flask import Flask, render_template, request, redirect, url_for, jsonify, send_file
from google.cloud import storage
import qrcode

app = Flask(__name__)

BUCKET_NAME = "script-resize"
INPUT_PREFIX = "img/before/"
OUTPUT_PREFIX = "img/after/"


def get_bucket():
    """Retourne le bucket GCS."""
    return storage.Client().bucket(BUCKET_NAME)


def list_after_images():
    """Liste les images dans img/after/ et extrait le nom de l'uploadeur.

    Convention de nommage : NomUploadeur_nomImage.ext
    On split sur le premier '_' pour récupérer le nom.
    """
    blobs = get_bucket().list_blobs(prefix=OUTPUT_PREFIX)
    images = []
    for blob in blobs:
        filename = blob.name.removeprefix(OUTPUT_PREFIX)
        if not filename:
            continue
        parts = filename.split("_", 1)
        uploader = parts[0] if len(parts) > 1 else "Inconnu"
        images.append({
            "filename": filename,
            "url": f"https://storage.googleapis.com/{BUCKET_NAME}/{OUTPUT_PREFIX}{filename}",
            "uploader": uploader,
        })
    return images


@app.route("/")
def index():
    return redirect(url_for("home"))


@app.route("/home")
def home():
    return render_template("home.html")


@app.route("/upload", methods=["GET", "POST"])
def upload():
    if request.method == "POST":
        name = request.form.get("name", "").strip()
        file = request.files.get("image")

        if not name or not file or file.filename == "":
            return render_template("upload.html", error="Le nom et l'image sont obligatoires."), 400

        gcs_filename = f"{name}_{file.filename}"
        blob = get_bucket().blob(INPUT_PREFIX + gcs_filename)
        blob.upload_from_file(file.stream, content_type=file.content_type)

        return redirect(url_for("home"))

    return render_template("upload.html")


@app.route("/api/images")
def api_images():
    return jsonify(list_after_images())


@app.route("/qrcode")
def qr():
    target = request.host_url.rstrip("/") + "/home"

    img = qrcode.make(target)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    buf.seek(0)

    return send_file(buf, mimetype="image/png")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080, debug=True)
