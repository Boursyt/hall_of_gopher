let images = [];
let currentIndex = 0;
let autoPlay = true;
let autoPlayInterval = null;
const preloaded = new Map();

function imageListKey(list) {
    return list.map(img => img.filename).join("|");
}

function preloadImages(list) {
    list.forEach(img => {
        if (!preloaded.has(img.url)) {
            const el = new Image();
            el.src = img.url;
            preloaded.set(img.url, el);
        }
    });

    // Nettoyer les anciennes URLs qui ne sont plus dans la liste
    const currentURLs = new Set(list.map(img => img.url));
    for (const url of preloaded.keys()) {
        if (!currentURLs.has(url)) {
            preloaded.delete(url);
        }
    }
}

async function fetchImages() {
    try {
        const res = await fetch("/api/images");
        const data = await res.json();
        if (imageListKey(data) !== imageListKey(images)) {
            images = data;
            currentIndex = Math.min(currentIndex, Math.max(0, images.length - 1));
            preloadImages(images);
            renderSlide();
        } else {
            images = data;
            preloadImages(images);
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

function toggleAutoPlay() {
    const checkbox = document.getElementById("toggle-autoplay");
    autoPlay = checkbox.checked;
    if (autoPlay) {
        autoPlayInterval = setInterval(nextSlide, 3000);
    } else {
        clearInterval(autoPlayInterval);
    }
}

fetchImages();
setInterval(fetchImages, 2000);
autoPlayInterval = setInterval(nextSlide, 3000);
