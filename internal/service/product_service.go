package service

import (
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

type ProductService struct {
	productStore       *store.ProductStore
	componentStore     *store.ComponentStore
	productTestEnvStore *store.ProductTestEnvStore
}

func NewProductService() *ProductService {
	return &ProductService{
		productStore:       store.NewProductStore(),
		componentStore:     store.NewComponentStore(),
		productTestEnvStore: store.NewProductTestEnvStore(),
	}
}

// --- Product CRUD ---

func (s *ProductService) CreateProduct(name, description, currentVersion string) (*model.Product, error) {
	p := &model.Product{
		Name:           name,
		Description:    description,
		CurrentVersion: currentVersion,
	}
	if err := s.productStore.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) CreateProductWithEnv(name, description, currentVersion string, testEnvEnabled bool) (*model.Product, error) {
	p := &model.Product{
		Name:           name,
		Description:    description,
		CurrentVersion: currentVersion,
		TestEnvEnabled: testEnvEnabled,
	}
	if err := s.productStore.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) GetProduct(id uint) (*model.Product, error) {
	return s.productStore.GetByID(id)
}

func (s *ProductService) ListProducts(keyword string) ([]model.Product, error) {
	return s.productStore.List(keyword)
}

func (s *ProductService) UpdateProduct(id uint, name, description, currentVersion string) error {
	p, err := s.productStore.GetByID(id)
	if err != nil {
		return err
	}
	p.Name = name
	p.Description = description
	p.CurrentVersion = currentVersion
	return s.productStore.Update(p)
}

func (s *ProductService) UpdateProductWithEnv(id uint, name, description, currentVersion string, testEnvEnabled bool) error {
	p, err := s.productStore.GetByID(id)
	if err != nil {
		return err
	}
	p.Name = name
	p.Description = description
	p.CurrentVersion = currentVersion
	p.TestEnvEnabled = testEnvEnabled
	return s.productStore.Update(p)
}

func (s *ProductService) DeleteProduct(id uint) error {
	return s.productStore.Delete(id)
}

func (s *ProductService) CountProducts() (int64, error) {
	return s.productStore.Count()
}

// --- Component CRUD ---

func (s *ProductService) AddComponent(productID uint, name string, compType model.ComponentType, gitURL, jenkinsJobName, currentVersion string) (*model.Component, error) {
	c := &model.Component{
		ProductID:      productID,
		Name:           name,
		Type:           compType,
		GitURL:         gitURL,
		JenkinsJobName: jenkinsJobName,
		CurrentVersion: currentVersion,
	}
	if err := s.componentStore.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ProductService) GetComponent(id uint) (*model.Component, error) {
	return s.componentStore.GetByID(id)
}

func (s *ProductService) ListComponents(productID uint) ([]model.Component, error) {
	return s.componentStore.ListByProductID(productID)
}

func (s *ProductService) UpdateComponent(id uint, name string, compType model.ComponentType, gitURL, jenkinsJobName, currentVersion string) error {
	c, err := s.componentStore.GetByID(id)
	if err != nil {
		return err
	}
	c.Name = name
	c.Type = compType
	c.GitURL = gitURL
	c.JenkinsJobName = jenkinsJobName
	c.CurrentVersion = currentVersion
	return s.componentStore.Update(c)
}

func (s *ProductService) DeleteComponent(id uint) error {
	return s.componentStore.Delete(id)
}

func (s *ProductService) UpdateComponentBranchFilter(id uint, branchFilter string) error {
	c, err := s.componentStore.GetByID(id)
	if err != nil {
		return err
	}
	c.BranchFilter = branchFilter
	return s.componentStore.Update(c)
}

func (s *ProductService) CountComponents() (int64, error) {
	return s.componentStore.Count()
}

// --- Product-TestEnv associations ---

func (s *ProductService) SetProductTestEnvs(productID uint, testEnvIDs []uint) error {
	return s.productTestEnvStore.Replace(productID, testEnvIDs)
}

func (s *ProductService) ListProductTestEnvIDs(productID uint) ([]uint, error) {
	ptes, err := s.productTestEnvStore.ListByProductID(productID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(ptes))
	for i, pte := range ptes {
		ids[i] = pte.TestEnvID
	}
	return ids, nil
}
