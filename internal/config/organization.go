package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	OrgModePersonal = "personal"
	OrgModeSingle   = "single"
	OrgModeMulti    = "multi"

	DefaultInvitationDeliveryProfile = "authara-hosted"
	InvitationTargetHosted           = "authara_hosted"
	InvitationTargetExternal         = "external"
	InvitationTargetNone             = "none"
	InvitationEmailLinkOnly          = "link_only"
	InvitationEmailLinkAndCode       = "link_and_code"
	InvitationEmailCodeOnly          = "code_only"
	InvitationTokenQuery             = "query"
	InvitationTokenFragment          = "fragment"

	invitationDeliveryProfilesFileVersion = 1
	maxInvitationDeliveryProfilesFileSize = 1 << 20
)

var invitationDeliveryProfileKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type InvitationDeliveryProfile struct {
	Target         string `yaml:"target"`
	URL            string `yaml:"url,omitempty"`
	EmailAction    string `yaml:"email_action"`
	TokenTransport string `yaml:"token_transport,omitempty"`
}

type invitationDeliveryProfilesDocument struct {
	Version  int                                  `yaml:"version"`
	Default  string                               `yaml:"default"`
	Profiles map[string]InvitationDeliveryProfile `yaml:"profiles"`
}

type Organization struct {
	Mode                                string `env:"AUTHARA_ORG_MODE,default=single"`
	PublicOrganizationManagementEnabled bool   `env:"AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED,default=false"`
	InvitationTTLRaw                    string `env:"AUTHARA_ORGANIZATION_INVITATION_TTL,default=168h"`
	InvitationDeliveryProfilesFile      string `env:"AUTHARA_INVITATION_DELIVERY_PROFILES_FILE"`

	InvitationTTL                    time.Duration
	InvitationDeliveryProfiles       map[string]InvitationDeliveryProfile
	DefaultInvitationDeliveryProfile string
}

func (o *Organization) validate() error {
	switch o.Mode {
	case OrgModePersonal, OrgModeSingle, OrgModeMulti:
		return nil
	default:
		return fmt.Errorf("invalid AUTHARA_ORG_MODE %q (allowed: personal, single, multi)", o.Mode)
	}
}

func (o *Organization) parse() error {
	ttl, err := time.ParseDuration(o.InvitationTTLRaw)
	if err != nil {
		return fmt.Errorf("invalid AUTHARA_ORGANIZATION_INVITATION_TTL: %q", o.InvitationTTLRaw)
	}
	if ttl <= 0 {
		return fmt.Errorf("AUTHARA_ORGANIZATION_INVITATION_TTL must be greater than 0")
	}
	o.InvitationTTL = ttl
	if err := o.parseInvitationDeliveryProfiles(); err != nil {
		return err
	}
	return nil
}

func (o *Organization) parseInvitationDeliveryProfiles() error {
	profiles := map[string]InvitationDeliveryProfile{
		DefaultInvitationDeliveryProfile: {
			Target:         InvitationTargetHosted,
			EmailAction:    InvitationEmailLinkAndCode,
			TokenTransport: InvitationTokenQuery,
		},
	}
	document := invitationDeliveryProfilesDocument{Default: DefaultInvitationDeliveryProfile}
	path := strings.TrimSpace(o.InvitationDeliveryProfilesFile)
	if path != "" {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: %w", err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("stat AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: %w", err)
		}
		if info.Size() > maxInvitationDeliveryProfilesFileSize {
			return fmt.Errorf("AUTHARA_INVITATION_DELIVERY_PROFILES_FILE exceeds %d bytes", maxInvitationDeliveryProfilesFileSize)
		}

		decoder := yaml.NewDecoder(io.LimitReader(file, maxInvitationDeliveryProfilesFileSize+1))
		decoder.KnownFields(true)
		if err := decoder.Decode(&document); err != nil {
			return fmt.Errorf("parse AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return fmt.Errorf("parse AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: multiple YAML documents are not supported")
			}
			return fmt.Errorf("parse AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: %w", err)
		}
		if document.Version != invitationDeliveryProfilesFileVersion {
			return fmt.Errorf("AUTHARA_INVITATION_DELIVERY_PROFILES_FILE version must be %d", invitationDeliveryProfilesFileVersion)
		}
	}

	for key, profile := range document.Profiles {
		if key == DefaultInvitationDeliveryProfile {
			return fmt.Errorf("AUTHARA_INVITATION_DELIVERY_PROFILES_FILE profile key %q is reserved", key)
		}
		normalized, err := normalizeInvitationDeliveryProfile(key, profile)
		if err != nil {
			return fmt.Errorf("invalid AUTHARA_INVITATION_DELIVERY_PROFILES_FILE: %w", err)
		}
		profiles[key] = normalized
	}
	defaultProfile := strings.TrimSpace(document.Default)
	if defaultProfile == "" {
		defaultProfile = DefaultInvitationDeliveryProfile
	}
	if _, ok := profiles[defaultProfile]; !ok {
		return fmt.Errorf("AUTHARA_INVITATION_DELIVERY_PROFILES_FILE default profile %q is not configured", defaultProfile)
	}
	o.InvitationDeliveryProfiles = profiles
	o.DefaultInvitationDeliveryProfile = defaultProfile
	return nil
}

func normalizeInvitationDeliveryProfile(key string, profile InvitationDeliveryProfile) (InvitationDeliveryProfile, error) {
	if !invitationDeliveryProfileKeyPattern.MatchString(key) {
		return InvitationDeliveryProfile{}, fmt.Errorf("profile key %q must match %s", key, invitationDeliveryProfileKeyPattern)
	}
	profile.Target = strings.TrimSpace(profile.Target)
	profile.URL = strings.TrimSpace(profile.URL)
	profile.EmailAction = strings.TrimSpace(profile.EmailAction)
	profile.TokenTransport = strings.TrimSpace(profile.TokenTransport)
	if profile.TokenTransport == "" {
		profile.TokenTransport = InvitationTokenQuery
	}

	switch profile.Target {
	case InvitationTargetHosted:
		if profile.URL != "" || profile.TokenTransport != InvitationTokenQuery {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: authara_hosted cannot define url and requires query transport", key)
		}
	case InvitationTargetExternal:
		parsed, err := url.Parse(profile.URL)
		if err != nil || parsed.Host == "" || parsed.User != nil || !secureInvitationDestination(parsed) {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: external target requires HTTPS (HTTP is allowed only for loopback hosts)", key)
		}
		if profile.TokenTransport == InvitationTokenFragment && parsed.Fragment != "" {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: fragment transport requires a url without a fragment", key)
		}
	case InvitationTargetNone:
		if profile.URL != "" {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: none target must not define url", key)
		}
		profile.TokenTransport = InvitationTokenQuery
	default:
		return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: unsupported target %q", key, profile.Target)
	}

	switch profile.EmailAction {
	case InvitationEmailLinkOnly, InvitationEmailLinkAndCode:
		if profile.Target == InvitationTargetNone {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: %s requires a link target", key, profile.EmailAction)
		}
	case InvitationEmailCodeOnly:
		if profile.Target != InvitationTargetNone {
			return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: code_only requires target none", key)
		}
	default:
		return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: unsupported email_action %q", key, profile.EmailAction)
	}
	if profile.Target != InvitationTargetNone && profile.TokenTransport != InvitationTokenQuery && profile.TokenTransport != InvitationTokenFragment {
		return InvitationDeliveryProfile{}, fmt.Errorf("profile %q: unsupported token_transport %q", key, profile.TokenTransport)
	}
	return profile, nil
}

func secureInvitationDestination(destination *url.URL) bool {
	if destination.Scheme == "https" {
		return true
	}
	if destination.Scheme != "http" {
		return false
	}
	host := destination.Hostname()
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback()
}
