# Fase 3 — Arquitectura del Proyecto Go y Fundación Base

### A. Información General
- **Identificador:** FASE-03
- **Nombre de Fase:** Scaffolding del Proyecto Go y Fundación del Núcleo
- **Objetivo:** Inicializar la estructura modular de directorios en Go, configurar dependencias, herramientas de linting y pruebas, e implementar utilidades de bajo nivel del sistema (permisos y consola).
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-01, FASE-02.
- **Módulos originales involucrados:**
  - `fbchat_v2/_core/_permissions.py`
  - `fbchat_v2/_core/_console.py`
- **Archivos Go previstos:**
  - `pkg/core/permissions/permissions.go`
  - `pkg/core/permissions/permissions_windows.go`
  - `pkg/core/permissions/permissions_posix.go`
  - `pkg/core/console/console.go`
  - `pkg/core/logger/logger.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `_permissions.py:set_private_file_permissions(path)`:
    - En POSIX: Aplica `os.chmod(path, 0o600)` y al directorio `0o700`.
    - En Windows: Ejecuta comandos `icacls` para remover herencia (`/inheritance:r`) y conceder acceso exclusivo al usuario actual (`/grant:r %USERNAME%:(F)`).
  - `_console.py:configure_utf8_console()`:
    - En Windows, reconfigura `sys.stdout` y `sys.stderr` con codificación `utf-8` para soportar emojis y caracteres de localización sin fallar con `UnicodeEncodeError`.
- **Riesgos específicos:**
  - Fallos en llamadas de sistema de permisos en entornos contenerizados o en Windows si `icacls` no está en el `PATH`.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Uso de compilación condicional con build tags Go:
  - `//go:build windows` para llamadas con `golang.org/x/sys/windows` o `icacls`.
  - `//go:build !windows` para `os.Chmod(path, 0600)`.
- **Interfaces y estructuras Go:**
  ```go
  package permissions

  func SetPrivateFilePermissions(path string) error
  func CreatePrivateJSONFile(path string, template any) (bool, error)
  ```

---

### D. Tareas Ejecutables
- [ ] Configurar el árbol de paquetes en `pkg/core/`.
- [ ] Implementar `SetPrivateFilePermissions` con soporte nativo multiplataforma (Linux/macOS `0600` y Windows ACLs).
- [ ] Implementar `CreatePrivateJSONFile` con sincronización atómica a disco.
- [ ] Implementar `pkg/core/console.SafePrint()` para salida estándar en UTF-8 seguro.
- [ ] Configurar `.golangci.yml` con reglas estrictas de seguridad (`gosec`, `errcheck`, `govet`).
- [ ] Crear tests unitarios para creación de archivos seguros y verificación de permisos.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `set_private_file_permissions` | `_core/_permissions.py:set_private_file_permissions` | `pkg/core/permissions.SetPrivateFilePermissions()` | Test de permisos de archivo | PENDIENTE |
| `create_private_json_file` | `_core/_permissions.py:create_private_json_file` | `pkg/core/permissions.CreatePrivateJSONFile()` | Test de archivo creado | PENDIENTE |
| `safe_print` | `_core/_console.py:safe_print` | `pkg/core/console.SafePrint()` | Test de salida UTF-8 | PENDIENTE |

---

### F. Pruebas y Aceptación
- **Pruebas Unitarias:**
  - `TestSetPrivateFilePermissions_Posix`: Verifica permisos `0600`.
  - `TestSetPrivateFilePermissions_Windows`: Verifica ACLs de Windows.
  - `TestSafePrint_Emojis`: Verifica que cadenas con caracteres Unicode y emojis se impriman sin corrupción.

---

### G. Riesgos y Dependencias
- Manejo de permisos en sistemas de archivos NTFS vs Ext4.

---

### H. Definición de Terminado
La fase se considerará terminada cuando los paquetes de permisos y consola compilen en Linux y Windows, pasando todas las pruebas unitarias.
