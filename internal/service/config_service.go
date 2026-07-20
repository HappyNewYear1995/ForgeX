package service

import (
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
	"time"
)

// SysConfigField defines a form field for system configuration display.
type SysConfigField struct {
	Key         string
	Label       string
	Placeholder string
	Value       string
	IsPassword  bool
}

// SysConfigCategory defines a category of system configuration.
type SysConfigCategory struct {
	Category   string
	Title      string
	Icon       string
	Fields     []SysConfigField
	TestStatus string // "ok", "err", or "" (never tested)
	TestTime   string // formatted time of last test
}

type ConfigService struct {
	store        *store.ConfigItemStore
	sysStore     *store.SysConfigStore
	testEnvStore *store.TestEnvStore
}

func NewConfigService() *ConfigService {
	return &ConfigService{
		store:        store.NewConfigItemStore(),
		sysStore:     store.NewSysConfigStore(),
		testEnvStore: store.NewTestEnvStore(),
	}
}

// predefinedFields returns the field definitions for each system config category.
func predefinedFields() map[string][]SysConfigField {
	return map[string][]SysConfigField{
		"jenkins": {
			{Key: "url", Label: "Jenkins 地址", Placeholder: "http://jenkins.example.com"},
			{Key: "user", Label: "用户名", Placeholder: "admin"},
			{Key: "token", Label: "API Token", Placeholder: "your-api-token", IsPassword: true},
		},
		"gitea": {
			{Key: "url", Label: "Gitea 地址", Placeholder: "http://gitea.example.com"},
			{Key: "token", Label: "API Token", Placeholder: "your-api-token", IsPassword: true},
		},
	}
}

// GetSysConfigCategories returns all system config categories with current values.
func (s *ConfigService) GetSysConfigCategories() []SysConfigCategory {
	defs := predefinedFields()
	var categories []SysConfigCategory

	for cat, fields := range defs {
		title := cat
		icon := "⚙️"
		switch cat {
		case "jenkins":
			title = "Jenkins 配置"
			icon = "🔧"
		case "gitea":
			title = "Gitea 配置"
			icon = "🐙"
		}

		// Load current values
		items, _ := s.sysStore.ListByCategory(cat)
		valueMap := make(map[string]string)
		for _, item := range items {
			valueMap[item.Key] = item.Value
		}

		var enrichedFields []SysConfigField
		for _, f := range fields {
			f.Value = valueMap[f.Key]
			enrichedFields = append(enrichedFields, f)
		}

		categories = append(categories, SysConfigCategory{
			Category:   cat,
			Title:      title,
			Icon:       icon,
			Fields:     enrichedFields,
			TestStatus: valueMap["_test_status"],
			TestTime:   valueMap["_test_time"],
		})
	}
	return categories
}

// SaveSysConfig saves a batch of key-value pairs for a category.
func (s *ConfigService) SaveSysConfig(category string, pairs map[string]string) error {
	return s.sysStore.SaveBatch(category, pairs)
}

// GetSysConfigValue returns a single system config value.
func (s *ConfigService) GetSysConfigValue(category, key string) (string, error) {
	return s.sysStore.Get(category, key)
}

// SaveTestResult persists the API test result for a category.
func (s *ConfigService) SaveTestResult(category, status string) error {
	pairs := map[string]string{
		"_test_status": status,
		"_test_time":   time.Now().Format("2006-01-02 15:04:05"),
	}
	return s.sysStore.SaveBatch(category, pairs)
}

// GetTree builds the full config tree (root nodes with children nested).
func (s *ConfigService) GetTree() ([]model.ConfigItem, error) {
	roots, err := s.store.ListRoots()
	if err != nil {
		return nil, err
	}
	for i := range roots {
		children, _ := s.store.ListByParentID(roots[i].ID)
		roots[i].Children = children
	}
	return roots, nil
}

// GetAll returns a flat list of all config items.
func (s *ConfigService) GetAll() ([]model.ConfigItem, error) {
	return s.store.List()
}

func (s *ConfigService) GetByID(id uint) (*model.ConfigItem, error) {
	return s.store.GetByID(id)
}

func (s *ConfigService) Create(name, code, description string, parentID uint, sortOrder int) (*model.ConfigItem, error) {
	item := &model.ConfigItem{
		ParentID:    parentID,
		Name:        name,
		Code:        code,
		Description: description,
		SortOrder:   sortOrder,
	}
	if err := s.store.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ConfigService) Update(id uint, name, code, description string, sortOrder int) error {
	item, err := s.store.GetByID(id)
	if err != nil {
		return err
	}
	item.Name = name
	item.Code = code
	item.Description = description
	item.SortOrder = sortOrder
	return s.store.Update(item)
}

func (s *ConfigService) Delete(id uint) error {
	return s.store.Delete(id)
}

// ListTestEnvs TestEnv CRUD
func (s *ConfigService) ListTestEnvs() ([]model.TestEnv, error) {
	return s.testEnvStore.List()
}

func (s *ConfigService) CreateTestEnv(name, url string) (*model.TestEnv, error) {
	item := &model.TestEnv{
		Name: name,
		URL:  url,
	}
	if err := s.testEnvStore.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ConfigService) UpdateTestEnv(id uint, name, url string) error {
	item := &model.TestEnv{ID: id, Name: name, URL: url}
	return s.testEnvStore.Update(item)
}

func (s *ConfigService) DeleteTestEnv(id uint) error {
	return s.testEnvStore.Delete(id)
}

func (s *ConfigService) GetTestEnv(id uint) (*model.TestEnv, error) {
	return s.testEnvStore.GetByID(id)
}
