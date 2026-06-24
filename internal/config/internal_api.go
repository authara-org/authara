package config

type InternalAPI struct {
	Token string `env:"AUTHARA_INTERNAL_API_TOKEN"`
}

func (i *InternalAPI) validate() error {
	return nil
}
