package adminui

import "embed"

//go:embed components.tmpl pages/*.tmpl
var TemplatesFS embed.FS

//go:generate npx tailwindcss@v3.2.4 --config tailwind.config.js --input style.css --output static/style.css --minify
//go:embed static/*
var StaticFS embed.FS
