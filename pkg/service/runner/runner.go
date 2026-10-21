package runner

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/console"
	"github.com/bradez-rd/fbreego/pkg/core/logger"
	"github.com/bradez-rd/fbreego/pkg/service/api"
	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

// Run inicia el microservicio fbreego con todos sus subsistemas y gestiona las señales de terminación.
func Run() {
	fs := flag.NewFlagSet("fbreego", flag.ContinueOnError)

	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8000"
	}
	defaultSessions := os.Getenv("FBCHAT_SESSIONS_DIR")
	if defaultSessions == "" {
		defaultSessions = ".fbchat_sessions"
	}

	portFlag := fs.String("p", defaultPort, "Puerto TCP para escuchar peticiones HTTP")
	fs.StringVar(portFlag, "port", defaultPort, "Puerto TCP para escuchar peticiones HTTP")

	sessionsFlag := fs.String("s", defaultSessions, "Directorio de almacenamiento de sesiones cifradas")
	fs.StringVar(sessionsFlag, "sessions", defaultSessions, "Directorio de almacenamiento de sesiones cifradas")

	mediaFlag := fs.String("m", ".fbchat_media", "Directorio de almacenamiento de archivos multimedia")
	fs.StringVar(mediaFlag, "media", ".fbchat_media", "Directorio de almacenamiento de archivos multimedia")

	versionFlag := fs.Bool("v", false, "Muestra la versión de fbreego")
	fs.BoolVar(versionFlag, "version", false, "Muestra la versión de fbreego")

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "fbreego - Microservicio y cliente de alto rendimiento en Go para Facebook Messenger\n")
		fmt.Fprintf(os.Stdout, "Versión: %s (%s)\n\n", api.Version, api.ServiceName)
		fmt.Fprintf(os.Stdout, "Uso:\n  fbreego [opciones]\n\n")
		fmt.Fprintf(os.Stdout, "Opciones disponibles:\n")
		fmt.Fprintf(os.Stdout, "  -p, --port string      Puerto HTTP (defecto: %s o $PORT)\n", defaultPort)
		fmt.Fprintf(os.Stdout, "  -s, --sessions string  Directorio de sesiones (defecto: %s)\n", defaultSessions)
		fmt.Fprintf(os.Stdout, "  -m, --media string     Directorio multimedia (defecto: .fbchat_media)\n")
		fmt.Fprintf(os.Stdout, "  -v, --version          Muestra la versión del servicio\n")
		fmt.Fprintf(os.Stdout, "  -h, --help             Muestra este mensaje de ayuda\n\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if *versionFlag {
		fmt.Printf("fbreego v%s (%s)\n", api.Version, api.ServiceName)
		os.Exit(0)
	}

	port := *portFlag
	addr := ":" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateDir := *sessionsFlag
	mediaDir := *mediaFlag
	if err := api.ValidateRuntimeConfiguration(); err != nil {
		logger.Error("Configuración insegura de API: %v", err)
		os.Exit(1)
	}

	encStore := sessions.NewEncryptedSessionStore(stateDir)
	webhookStore := webhook.NewWebhookConfigStore(stateDir)
	if err := encStore.Ready(); err != nil {
		logger.Error("Configuración insegura del almacén de sesiones: %v", err)
		os.Exit(1)
	}
	if err := webhookStore.Ready(); err != nil {
		logger.Error("Configuración insegura del almacén de webhook: %v", err)
		os.Exit(1)
	}
	webhookDisp := dispatcher.NewDispatcher(webhookStore)
	eventHub := events.NewRealtimeHub()
	listenerMgr := events.NewSessionListenerManager(eventHub)
	mediaStore := media.NewMediaStorage(mediaDir)
	sessionReg := sessions.NewSessionRegistry(encStore, nil)
	if err := sessionReg.Ready(); err != nil {
		logger.Error("No se pudieron cargar las sesiones cifradas: %v", err)
		os.Exit(1)
	}

	server := api.NewServer(
		sessionReg,
		webhookStore,
		webhookDisp,
		mediaStore,
		eventHub,
		listenerMgr,
	)

	webhookDisp.Start(ctx)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	go func() {
		console.SafePrintf(os.Stdout, "🚀 fbreego (%s v%s) escuchando en http://localhost:%s\n", api.ServiceName, api.Version, port)
		console.SafePrintf(os.Stdout, "   Portal administrativo: http://localhost:%s/\n", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Fallo crítico en el servidor HTTP: %v", err)
			os.Exit(1)
		}
	}()

	<-stopSig
	console.SafePrint(os.Stdout, "\n🛑 Recibida señal de apagado, cerrando servicios limpiamente...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	listenerMgr.StopAll()
	webhookDisp.Stop()
	_ = httpServer.Shutdown(shutdownCtx)

	console.SafePrint(os.Stdout, "✅ fbreego se ha detenido correctamente.")
}
