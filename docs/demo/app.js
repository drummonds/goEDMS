// godocs WASM demo - client-side navigation interceptor

(function() {
    "use strict";

    const appEl = document.getElementById("app");
    const loadingEl = document.getElementById("loading");
    const statusEl = document.getElementById("status");
    const progressEl = document.getElementById("progress");

    // Called by Go WASM when ready
    window.wasmReady = function() {
        loadingEl.style.display = "none";
        appEl.classList.add("loaded");
        // Load the home page
        navigate("/", "GET");
    };

    // Navigate to a URL using the WASM handler
    function navigate(path, method, body, contentType) {
        method = method || "GET";
        body = body || "";
        contentType = contentType || "";

        var result = window.handleRequest(method, path, body, contentType);
        if (!result) return;

        var status = result.status;
        var headers = result.headers || {};
        var respBody = result.body || "";

        // Follow redirects
        if ((status === 302 || status === 303 || status === 301) && headers["location"]) {
            navigate(headers["location"], "GET");
            return;
        }

        // Update the page content
        appEl.innerHTML = respBody;

        // Update URL (only for GET)
        if (method === "GET") {
            history.pushState({ path: path }, "", path);
        }

        // Post-render: fix thumbnail images
        fixThumbnails();

        // Re-attach event listeners
        attachListeners();

        // Scroll to top
        window.scrollTo(0, 0);
    }

    // Fix thumbnail images by loading them via WASM
    function fixThumbnails() {
        var imgs = appEl.querySelectorAll("img[src*='/api/document/']");
        for (var i = 0; i < imgs.length; i++) {
            var img = imgs[i];
            var src = img.getAttribute("src");
            if (src && src.indexOf("/thumbnail") > -1) {
                loadThumbnail(img, src);
            }
        }
    }

    function loadThumbnail(img, src) {
        var result = window.handleRequest("GET", src, "", "");
        if (result && result.isBytes && result.bodyBytes) {
            var blob = new Blob([result.bodyBytes], { type: "image/png" });
            img.src = URL.createObjectURL(blob);
        }
    }

    // Attach event listeners to intercept navigation
    function attachListeners() {
        // Intercept link clicks
        var links = appEl.querySelectorAll("a[href]");
        for (var i = 0; i < links.length; i++) {
            links[i].addEventListener("click", handleLinkClick);
        }

        // Intercept form submissions
        var forms = appEl.querySelectorAll("form");
        for (var i = 0; i < forms.length; i++) {
            forms[i].addEventListener("submit", handleFormSubmit);
        }
    }

    function handleLinkClick(e) {
        var href = this.getAttribute("href");
        if (!href) return;

        // Skip external links
        if (href.indexOf("http://") === 0 || href.indexOf("https://") === 0) return;
        // Skip anchor links
        if (href.indexOf("#") === 0) return;
        // Skip document view links (not supported in demo)
        if (href.indexOf("/document/view/") === 0) {
            e.preventDefault();
            alert("Document viewing is not available in the demo.");
            return;
        }

        e.preventDefault();
        navigate(href, "GET");
    }

    function handleFormSubmit(e) {
        e.preventDefault();
        var form = e.target;
        var method = (form.method || "GET").toUpperCase();
        var action = form.action ? new URL(form.action).pathname : window.location.pathname;

        if (method === "GET") {
            var params = new URLSearchParams(new FormData(form)).toString();
            var url = action + (params ? "?" + params : "");
            navigate(url, "GET");
        } else {
            var formData = new URLSearchParams(new FormData(form)).toString();
            navigate(action, method, formData, "application/x-www-form-urlencoded");
        }
    }

    // Handle browser back/forward
    window.addEventListener("popstate", function(e) {
        var path = window.location.pathname + window.location.search;
        navigate(path, "GET");
    });

    // Load WASM
    function loadWASM() {
        statusEl.textContent = "Downloading WASM binary...";
        progressEl.value = 10;

        var go = new Go();

        fetch("godocs-demo.wasm")
            .then(function(response) {
                if (!response.ok) throw new Error("Failed to fetch WASM: " + response.status);
                statusEl.textContent = "Compiling WebAssembly...";
                progressEl.value = 50;
                return WebAssembly.instantiateStreaming(response, go.importObject);
            })
            .then(function(result) {
                statusEl.textContent = "Starting godocs...";
                progressEl.value = 80;
                go.run(result.instance);
            })
            .catch(function(err) {
                statusEl.textContent = "Error: " + err.message;
                progressEl.classList.remove("is-primary");
                progressEl.classList.add("is-danger");
                console.error("WASM load error:", err);
            });
    }

    loadWASM();
})();
