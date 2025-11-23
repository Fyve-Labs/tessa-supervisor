package remote_commands

type BeszelConfig struct {
	ServerURL    string `json:"server_url"`
	Token        string `json:"token"`
	SSHPublicKey string `json:"ssh_public_key"`
}
