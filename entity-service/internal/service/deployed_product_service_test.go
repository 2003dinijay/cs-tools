// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubDeployedProductRepo is a minimal repository.DeployedProductRepository
// whose SearchDeployedProducts panics if called: tests using it prove the
// PostgreSQL-backed service rejects an unsupported ProductCategories filter
// before ever reaching the repository, not merely that the repository
// ignores it.
type stubDeployedProductRepo struct {
	searchDeployedProducts func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)
}

func (s *stubDeployedProductRepo) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
	if s.searchDeployedProducts != nil {
		return s.searchDeployedProducts(ctx, req)
	}
	panic("SearchDeployedProducts called unexpectedly: the productCategories rejection should have short-circuited before reaching the repository")
}

func TestDeployedProductService_SearchDeployedProducts_RejectsProductCategories(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{
		ProductCategories: []string{"pdp"},
	})

	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestDeployedProductService_SearchDeployedProducts_NoProductCategoriesReachesRepository(t *testing.T) {
	called := false
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			called = true
			return nil, 0, nil
		},
	})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected SearchDeployedProducts to reach the repository when ProductCategories is empty")
	}
}

// TestDeployedProductService_SearchProjectsByProductVersion_RejectedOnPostgres
// guards the PostgreSQL data source's rejection of the EOL/product-version
// reverse query, which it has no deployment/deployed-product model rich
// enough to resolve. The repository is never consulted, so a nil repo is
// safe here.
func TestDeployedProductService_SearchProjectsByProductVersion_RejectedOnPostgres(t *testing.T) {
	svc := NewDeployedProductService(nil)

	_, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "11111111-1111-1111-1111-111111111111",
		ProductVersionID: "22222222-2222-2222-2222-222222222222",
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}
