package manifests

type HubConfig struct {
	ClusterManagerName             string
	ClusterManagerNamespace        string
	RegistrationImage              string
	RegistrationAPIServiceCABundle string
	WorkImage                      string
	WorkAPIServiceCABundle         string
	PlacementImage                 string
	Replica                        int32
	HostedMode                     bool
	RegistrationWebhook            Webhook
	WorkWebhook                    Webhook
}

type Webhook struct {
	IsIPFormat bool
	Port       int32
	Address    string
}
