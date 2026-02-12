let images = [];
let currentIndex = 0;

async function fetchImages() {
    try {
        const res = await fetch("/api/images");
        const data = await res.json();
        if (JSON.stringify(data) !== JSON.stringify(images)) {
            images = data;
            currentIndex = Math.min(currentIndex, Math.max(0, images.length - 1));
            renderSlide();
        }
    } catch (e) {
        console.error("Erreur fetch images:", e);
    }
}

function renderSlide() {
    const slider = document.querySelector(".slider");
    const noImg = document.getElementById("no-images");

    slider.querySelectorAll(".slide").forEach(el => el.remove());

    if (images.length === 0) {
        noImg.style.display = "block";
        return;
    }
    noImg.style.display = "none";

    const img = images[currentIndex];
    const slide = document.createElement("div");
    slide.className = "slide";
    slide.innerHTML = `
        <img src="${img.url}" alt="${img.filename}">
        <p class="uploader">Par : ${img.uploader}</p>
        <p class="counter">${currentIndex + 1} / ${images.length}</p>
    `;
    slider.appendChild(slide);
}

function nextSlide() {
    if (images.length === 0) return;
    currentIndex = (currentIndex + 1) % images.length;
    renderSlide();
}

function prevSlide() {
    if (images.length === 0) return;
    currentIndex = (currentIndex - 1 + images.length) % images.length;
    renderSlide();
}

fetchImages();
setInterval(fetchImages, 5000);
setInterval(nextSlide, 3000);
