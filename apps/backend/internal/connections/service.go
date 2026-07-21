// Package connections manages stored Active Directory connection profiles.
// Passwords are encrypted at rest via internal/secrets and only decrypted
// server-side when a handler needs to bind to AD.
package connections

import (
	"errors"
	"strings"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/secrets"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound     = errors.New("connection profile not found")
	ErrNameRequired = errors.New("connection profile name is required")
	ErrInvalidInput = errors.New("server and bind_user are required")
)

// Service persists and resolves AD connection profiles.
type Service struct {
	db      *gorm.DB
	dataDir string
}

func NewService(db *gorm.DB, dataDir string) *Service {
	return &Service{db: db, dataDir: dataDir}
}

// ProfileInput is the write payload. Password may be empty on update to keep
// the existing stored password.
type ProfileInput struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	BaseDN    string `json:"base_dn"`
	BindUser  string `json:"bind_user"`
	Password  string `json:"password"`
	IsDefault bool   `json:"is_default"`
}

// ResolvedCredentials carries decrypted bind credentials; never serialize.
type ResolvedCredentials struct {
	Server   string
	BaseDN   string
	BindUser string
	Password string
}

func (input ProfileInput) validate(requirePassword bool) error {
	if strings.TrimSpace(input.Name) == "" {
		return ErrNameRequired
	}
	// BaseDN is optional — the API layer auto-discovers it from the RootDSE
	// when omitted, so operators only need server + account + password.
	if strings.TrimSpace(input.Server) == "" || strings.TrimSpace(input.BindUser) == "" {
		return ErrInvalidInput
	}
	if requirePassword && input.Password == "" {
		return errors.New("password is required when creating a connection profile")
	}
	return nil
}

func (service *Service) List() ([]models.ADConnectionProfile, error) {
	var profiles []models.ADConnectionProfile
	err := service.db.Order("is_default DESC, name ASC").Find(&profiles).Error
	return profiles, err
}

func (service *Service) Get(id uuid.UUID) (*models.ADConnectionProfile, error) {
	var profile models.ADConnectionProfile
	err := service.db.First(&profile, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// Default returns the default profile, or the only profile when just one exists.
func (service *Service) Default() (*models.ADConnectionProfile, error) {
	var profile models.ADConnectionProfile
	err := service.db.First(&profile, "is_default = ?", true).Error
	if err == nil {
		return &profile, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var profiles []models.ADConnectionProfile
	if err := service.db.Limit(2).Find(&profiles).Error; err != nil {
		return nil, err
	}
	if len(profiles) == 1 {
		return &profiles[0], nil
	}
	return nil, ErrNotFound
}

func (service *Service) Create(input ProfileInput) (*models.ADConnectionProfile, error) {
	if err := input.validate(true); err != nil {
		return nil, err
	}

	key, err := secrets.LoadOrCreateKey(service.dataDir)
	if err != nil {
		return nil, err
	}
	encrypted, err := secrets.Encrypt(key, input.Password)
	if err != nil {
		return nil, err
	}

	profile := models.ADConnectionProfile{
		Name:              strings.TrimSpace(input.Name),
		Server:            strings.TrimSpace(input.Server),
		BaseDN:            strings.TrimSpace(input.BaseDN),
		BindUser:          strings.TrimSpace(input.BindUser),
		EncryptedPassword: encrypted,
		IsDefault:         input.IsDefault,
	}

	err = service.db.Transaction(func(tx *gorm.DB) error {
		if profile.IsDefault {
			if err := tx.Model(&models.ADConnectionProfile{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&profile).Error
	})
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (service *Service) Update(id uuid.UUID, input ProfileInput) (*models.ADConnectionProfile, error) {
	if err := input.validate(false); err != nil {
		return nil, err
	}

	profile, err := service.Get(id)
	if err != nil {
		return nil, err
	}

	profile.Name = strings.TrimSpace(input.Name)
	profile.Server = strings.TrimSpace(input.Server)
	profile.BaseDN = strings.TrimSpace(input.BaseDN)
	profile.BindUser = strings.TrimSpace(input.BindUser)
	profile.IsDefault = input.IsDefault

	if input.Password != "" {
		key, err := secrets.LoadOrCreateKey(service.dataDir)
		if err != nil {
			return nil, err
		}
		encrypted, err := secrets.Encrypt(key, input.Password)
		if err != nil {
			return nil, err
		}
		profile.EncryptedPassword = encrypted
	}

	err = service.db.Transaction(func(tx *gorm.DB) error {
		if profile.IsDefault {
			if err := tx.Model(&models.ADConnectionProfile{}).Where("id <> ? AND is_default = ?", profile.ID, true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(profile).Error
	})
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (service *Service) Delete(id uuid.UUID) error {
	result := service.db.Delete(&models.ADConnectionProfile{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Resolve decrypts the stored credentials for a profile so a handler can bind.
func (service *Service) Resolve(id uuid.UUID) (*ResolvedCredentials, error) {
	profile, err := service.Get(id)
	if err != nil {
		return nil, err
	}
	return service.resolveProfile(profile)
}

// ResolveDefault resolves the default profile's credentials.
func (service *Service) ResolveDefault() (*ResolvedCredentials, error) {
	profile, err := service.Default()
	if err != nil {
		return nil, err
	}
	return service.resolveProfile(profile)
}

func (service *Service) resolveProfile(profile *models.ADConnectionProfile) (*ResolvedCredentials, error) {
	key, err := secrets.LoadOrCreateKey(service.dataDir)
	if err != nil {
		return nil, err
	}
	password, err := secrets.Decrypt(key, profile.EncryptedPassword)
	if err != nil {
		return nil, err
	}
	return &ResolvedCredentials{
		Server:   profile.Server,
		BaseDN:   profile.BaseDN,
		BindUser: profile.BindUser,
		Password: password,
	}, nil
}

// RecordTestResult stores the outcome of a connection test on the profile.
func (service *Service) RecordTestResult(id uuid.UUID, ok bool) error {
	return service.db.Model(&models.ADConnectionProfile{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_tested_at": gorm.Expr("CURRENT_TIMESTAMP"),
			"last_test_ok":   ok,
		}).Error
}
