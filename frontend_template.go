package main

import (
	"fmt"

	"github.com/drummonds/godocs/config"
)

// GetFrontendHTMLTemplate returns the HTML template for the frontend shell
// It dynamically detects the backend URL from where wasm_exec.js is loaded
func GetFrontendHTMLTemplate(serverConfig config.ServerConfig) string {
	// Use localhost as default if no IP is configured
	backendHost := serverConfig.ListenAddrIP
	if backendHost == "" {
		backendHost = "localhost"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs</title>
    <meta name="description" content="Electronic Document Management System">
    <script>
        // Detect backend URL - will be set after wasm_exec.js loads
        window.godocsBackendURL = null;

        function getBackendURL() {
            // Find wasm_exec.js script tag and extract its source URL
            const scripts = document.getElementsByTagName('script');
            for (let script of scripts) {
                if (script.src && script.src.includes('wasm_exec.js')) {
                    // Extract base URL (everything before /wasm_exec.js)
                    const url = new URL(script.src);
                    window.godocsBackendURL = url.origin;
                    console.log('Backend URL detected from wasm_exec.js:', window.godocsBackendURL);
                    return window.godocsBackendURL;
                }
            }
            // Fallback: use configured backend URL
            window.godocsBackendURL = 'http://' + window.location.hostname + ':%s';
            console.log('Backend URL fallback:', window.godocsBackendURL);
            return window.godocsBackendURL;
        }
    </script>
    <script src="http://%s:%s/wasm_exec.js" onload="getBackendURL()"></script>
    <script>
        // After wasm_exec.js loads, dynamically load remaining resources
        document.addEventListener('DOMContentLoaded', function() {
            const backendURL = window.godocsBackendURL || getBackendURL();

            // Load favicon
            const favicon = document.createElement('link');
            favicon.rel = 'icon';
            favicon.href = backendURL + '/favicon.ico';
            document.head.appendChild(favicon);

            // Load stylesheets
            const css1 = document.createElement('link');
            css1.rel = 'stylesheet';
            css1.href = backendURL + '/webapp/webapp.css';
            document.head.appendChild(css1);

            const css2 = document.createElement('link');
            css2.rel = 'stylesheet';
            css2.href = backendURL + '/webapp/wordcloud.css';
            document.head.appendChild(css2);

            // Set godocsConfig if not already set by config.js
            if (!window.godocsConfig) {
                window.godocsConfig = {
                    apiURL: backendURL,
                    newDocumentCount: %d
                };
            }

            // Load and run the WASM application
            const go = new Go();
            WebAssembly.instantiateStreaming(
                fetch(backendURL + '/web/app.wasm'),
                go.importObject
            ).then((result) => {
                go.run(result.instance);
            }).catch((err) => {
                console.error('Failed to load WASM:', err);
            });
        });
    </script>
</head>
<body>
    <div id="app"></div>
</body>
</html>`, serverConfig.ListenAddrPort, backendHost, serverConfig.ListenAddrPort, serverConfig.NewDocumentNumber)
}
