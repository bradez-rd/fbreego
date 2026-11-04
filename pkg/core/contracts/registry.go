package contracts

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Endpoints canónicos de Facebook utilizados por el servicio fbreego.
const (
	DefaultFacebookBaseURL       = "https://www.facebook.com"
	DefaultGraphQLURL            = "https://www.facebook.com/api/graphql/"
	DefaultBatchQueryURL         = "https://www.facebook.com/api/graphqlbatch/"
	DefaultSendURL               = "https://www.facebook.com/messaging/send/"
	DefaultUnsendURL             = "https://www.facebook.com/messaging/unsend_message/"
	DefaultMercuryUploadURL      = "https://upload.facebook.com/ajax/mercury/upload.php"
	DefaultMessagesBaseURL       = "https://www.facebook.com/messages/t/"
	DefaultSaveThreadNameURL     = "https://www.facebook.com/messaging/set_thread_name/"
	DefaultSaveThreadEmojiURL    = "https://www.facebook.com/messaging/save_thread_emoji/"
	DefaultSaveThreadNicknameURL = "https://www.facebook.com/messaging/save_thread_nickname/"
	DefaultSaveAdminsURL         = "https://www.facebook.com/messaging/save_admins/"
	DefaultRemoveFriendURL       = "https://www.facebook.com/ajax/profile/removefriendconfirm.php"
	DefaultAuthLoginURL          = "https://b-graph.facebook.com/auth/login"
)

// DocIDs y VersionIDs canónicos de las operaciones de Facebook Messenger conocidas.
const (
	DefaultDocIDThreadList          = "2289680371118670"
	DefaultDocIDMessageRequests     = "2289680371118670"
	DefaultDocIDMessagingThemeQuery = "1761643933953838"
	DefaultDocIDProfessionalMode    = "4920194019284712"
	DefaultDocIDFeedStoryArchive    = "5129402940471201"
	DefaultDocIDPrivacyBlockUser    = "3829104820491023"
	DefaultVersionIDEditMessage     = "7191105584282302"
)

// OperationContract encapsula los metadatos de un contrato de endpoint/operación privada de Facebook.
type OperationContract struct {
	FriendlyName string `json:"friendlyName"`
	DocID        string `json:"docId"`
	Endpoint     string `json:"endpoint"`
	Description  string `json:"description"`
}

// Registry centraliza los contratos, identificadores privados y URLs de Facebook,
// permitiendo sobrescrituras dinámicas mediante variables de entorno o configuración en runtime.
type Registry struct {
	mu         sync.RWMutex
	operations map[string]OperationContract
	endpoints  map[string]string
}

var (
	defaultRegistry *Registry
	onceRegistry    sync.Once
)

// DefaultRegistry devuelve la instancia singleton del registro centralizado de contratos.
func DefaultRegistry() *Registry {
	onceRegistry.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// NewRegistry inicializa un registro poblado con los contratos y valores por defecto.
func NewRegistry() *Registry {
	r := &Registry{
		operations: make(map[string]OperationContract),
		endpoints:  make(map[string]string),
	}

	// Registrar endpoints canónicos
	r.endpoints["base"] = DefaultFacebookBaseURL
	r.endpoints["graphql"] = DefaultGraphQLURL
	r.endpoints["graphqlbatch"] = DefaultBatchQueryURL
	r.endpoints["send"] = DefaultSendURL
	r.endpoints["unsend"] = DefaultUnsendURL
	r.endpoints["mercury_upload"] = DefaultMercuryUploadURL
	r.endpoints["messages"] = DefaultMessagesBaseURL
	r.endpoints["thread_name"] = DefaultSaveThreadNameURL
	r.endpoints["thread_emoji"] = DefaultSaveThreadEmojiURL
	r.endpoints["thread_nickname"] = DefaultSaveThreadNicknameURL
	r.endpoints["thread_admins"] = DefaultSaveAdminsURL
	r.endpoints["remove_friend"] = DefaultRemoveFriendURL
	r.endpoints["auth_login"] = DefaultAuthLoginURL

	// Registrar operaciones tipadas conocidas
	r.operations["ThreadListQuery"] = OperationContract{
		FriendlyName: "ThreadListQuery",
		DocID:        DefaultDocIDThreadList,
		Endpoint:     DefaultBatchQueryURL,
		Description:  "Consulta GraphQLBatch para obtener conversaciones en bandeja de entrada (INBOX)",
	}
	r.operations["MessageRequestsQuery"] = OperationContract{
		FriendlyName: "MessageRequestsQuery",
		DocID:        DefaultDocIDMessageRequests,
		Endpoint:     DefaultBatchQueryURL,
		Description:  "Consulta GraphQLBatch para obtener solicitudes de mensajes pendientes o archivados",
	}
	r.operations["MessagingThemeQuery"] = OperationContract{
		FriendlyName: "MessagingThemeQuery",
		DocID:        DefaultDocIDMessagingThemeQuery,
		Endpoint:     DefaultGraphQLURL,
		Description:  "Consulta GraphQL para listar temas visuales de Messenger",
	}
	r.operations["ProfessionalModeMutation"] = OperationContract{
		FriendlyName: "ProfessionalModeMutation",
		DocID:        DefaultDocIDProfessionalMode,
		Endpoint:     DefaultGraphQLURL,
		Description:  "Mutación GraphQL para consultar o alternar modo profesional de perfil",
	}
	r.operations["FeedStoryArchiveMutation"] = OperationContract{
		FriendlyName: "FeedStoryArchiveMutation",
		DocID:        DefaultDocIDFeedStoryArchive,
		Endpoint:     DefaultGraphQLURL,
		Description:  "Mutación GraphQL para gestionar publicaciones archivadas",
	}
	r.operations["PrivacyBlockUserMutation"] = OperationContract{
		FriendlyName: "PrivacyBlockUserMutation",
		DocID:        DefaultDocIDPrivacyBlockUser,
		Endpoint:     DefaultGraphQLURL,
		Description:  "Mutación GraphQL para bloquear o desbloquear a un usuario",
	}
	r.operations["EditMessageVersion"] = OperationContract{
		FriendlyName: "EditMessageVersion",
		DocID:        DefaultVersionIDEditMessage,
		Endpoint:     DefaultSendURL,
		Description:  "Identificador de versión de payload para edición de mensajes LightSpeed",
	}

	return r
}

// GetDocID obtiene el DocID asociado a una operación, respetando overrides por variables de entorno.
func (r *Registry) GetDocID(friendlyName string) string {
	envKey := "FBCHAT_DOCID_" + strings.ToUpper(strings.ReplaceAll(friendlyName, "-", "_"))
	if envVal := strings.TrimSpace(os.Getenv(envKey)); envVal != "" {
		return envVal
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if op, ok := r.operations[friendlyName]; ok && op.DocID != "" {
		return op.DocID
	}
	return ""
}

// GetEndpoint obtiene una URL de endpoint canónica, respetando overrides por entorno.
func (r *Registry) GetEndpoint(name string) string {
	envKey := "FBCHAT_URL_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if envVal := strings.TrimSpace(os.Getenv(envKey)); envVal != "" {
		return envVal
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if u, ok := r.endpoints[name]; ok && u != "" {
		return u
	}
	return ""
}

// RegisterOperation registra o actualiza un contrato de operación GraphQL en tiempo de ejecución.
func (r *Registry) RegisterOperation(op OperationContract) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.operations[op.FriendlyName] = op
}

// SetEndpoint actualiza la URL de un endpoint en tiempo de ejecución.
func (r *Registry) SetEndpoint(name, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints[name] = url
}

// Helpers de acceso global rápido mediante DefaultRegistry:

func GraphQLURL() string {
	if u := DefaultRegistry().GetEndpoint("graphql"); u != "" {
		return u
	}
	return DefaultGraphQLURL
}

func BatchQueryURL() string {
	if u := DefaultRegistry().GetEndpoint("graphqlbatch"); u != "" {
		return u
	}
	return DefaultBatchQueryURL
}

func SendURL() string {
	if u := DefaultRegistry().GetEndpoint("send"); u != "" {
		return u
	}
	return DefaultSendURL
}

func UnsendURL() string {
	if u := DefaultRegistry().GetEndpoint("unsend"); u != "" {
		return u
	}
	return DefaultUnsendURL
}

func MercuryUploadURL() string {
	if u := DefaultRegistry().GetEndpoint("mercury_upload"); u != "" {
		return u
	}
	return DefaultMercuryUploadURL
}

func MessagesBaseURL() string {
	if u := DefaultRegistry().GetEndpoint("messages"); u != "" {
		return u
	}
	return DefaultMessagesBaseURL
}

func SaveThreadNameURL() string {
	if u := DefaultRegistry().GetEndpoint("thread_name"); u != "" {
		return u
	}
	return DefaultSaveThreadNameURL
}

func SaveThreadEmojiURL() string {
	if u := DefaultRegistry().GetEndpoint("thread_emoji"); u != "" {
		return u
	}
	return DefaultSaveThreadEmojiURL
}

func SaveThreadNicknameURL() string {
	if u := DefaultRegistry().GetEndpoint("thread_nickname"); u != "" {
		return u
	}
	return DefaultSaveThreadNicknameURL
}

func SaveAdminsURL() string {
	if u := DefaultRegistry().GetEndpoint("thread_admins"); u != "" {
		return u
	}
	return DefaultSaveAdminsURL
}

func RemoveFriendURL() string {
	if u := DefaultRegistry().GetEndpoint("remove_friend"); u != "" {
		return u
	}
	return DefaultRemoveFriendURL
}

func DocID(friendlyName string) string {
	return DefaultRegistry().GetDocID(friendlyName)
}

func GetOperation(friendlyName string) (OperationContract, error) {
	r := DefaultRegistry()
	r.mu.RLock()
	defer r.mu.RUnlock()
	op, ok := r.operations[friendlyName]
	if !ok {
		return OperationContract{}, fmt.Errorf("operación %s no registrada en el catálogo de contratos", friendlyName)
	}
	// Aplicar override si existe
	if override := r.GetDocID(friendlyName); override != "" {
		op.DocID = override
	}
	return op, nil
}
