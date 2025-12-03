package main

import (
	"fmt"

	"github.com/drummonds/godocs/config"
)

// GetFrontendHTMLTemplate returns the HTML template for the frontend shell
// The backend URL is set directly so document links point to the backend
func GetFrontendHTMLTemplate(serverConfig config.ServerConfig) string {
	// Use localhost as default if no IP is configured
	backendHost := serverConfig.ListenAddrIP
	if backendHost == "" {
		backendHost = "localhost"
	}
	backendURL := fmt.Sprintf("http://%s:%s", backendHost, serverConfig.ListenAddrPort)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs</title>
    <meta name="description" content="Electronic Document Management System">
    <link rel="icon" href="%s/favicon.ico">
    <link rel="stylesheet" href="%s/webapp/webapp.css">
    <link rel="stylesheet" href="%s/webapp/wordcloud.css">
    <script>
        // Set backend URL directly - document view links will use this
        window.godocsBackendURL = '%s';
        window.godocsConfig = {
            apiURL: '%s',
            newDocumentCount: %d
        };
    </script>
    <script src="%s/wasm_exec.js"></script>
</head>
<body>
    <div id="app"></div>
    <script>
        // Load and run the WASM application
        const go = new Go();
        WebAssembly.instantiateStreaming(
            fetch("%s/web/app.wasm"),
            go.importObject
        ).then((result) => {
            go.run(result.instance);
        }).catch((err) => {
            console.error("Failed to load WASM:", err);
        });
    </script>
</body>
</html>`, backendURL, backendURL, backendURL, backendURL, backendURL, serverConfig.NewDocumentNumber, backendURL, backendURL)
}
