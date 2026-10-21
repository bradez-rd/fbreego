package console

import (
	"fmt"
	"io"
	"os"
)

// SafePrint imprime un mensaje de forma segura en UTF-8 hacia la salida especificada (o os.Stdout por defecto).
func SafePrint(w io.Writer, message string) {
	if w == nil {
		w = os.Stdout
	}
	_, _ = fmt.Fprintln(w, message)
}

// SafePrintf formatea e imprime un mensaje de forma segura en UTF-8 hacia la salida especificada.
func SafePrintf(w io.Writer, format string, args ...any) {
	if w == nil {
		w = os.Stdout
	}
	_, _ = fmt.Fprintf(w, format, args...)
}
