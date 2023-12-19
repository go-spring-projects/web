package api

import (
	"context"
	"math/rand"
	"net/http"

	"examples/petstore/models"
	"go-spring.dev/web"
)

func Register(router web.Router) {

	// Pet
	router.Group("/pet", func(r web.Router) {
		petRepo := NewPetRepository()

		r.Post("/", petRepo.AddPet)
		r.Get("/{petID}", petRepo.FindPet)
		r.Delete("/{petID}", petRepo.DeletePet)
		r.Get("/findByStatus", petRepo.FindPetByStatus)
	})

	// TODO: Store, User
}

func NewPetRepository() *PetRepository {
	return &PetRepository{memoryStore: make(map[int]models.Pet)}
}

type PetRepository struct {
	memoryStore map[int]models.Pet
}

func (pr *PetRepository) AddPet(
	ctx context.Context,
	req models.Pet,
) models.Pet {
	// generate pet id
	req.ID = rand.Int()
	// store pet
	pr.memoryStore[req.ID] = req
	return req
}

func (pr *PetRepository) FindPet(
	ctx context.Context,
	req struct {
		PetID int `path:"petID"`
	},
) (*models.Pet, error) {
	pet, ok := pr.memoryStore[req.PetID]
	if !ok {
		return nil, web.Error(http.StatusNotFound, "Pet not found")
	}
	return &pet, nil
}

func (pr *PetRepository) DeletePet(
	ctx context.Context,
	req struct {
		PetID int `path:"petID"`
	},
) error {
	pet, ok := pr.memoryStore[req.PetID]
	if !ok {
		return web.Error(http.StatusNotFound, "Pet not found")
	}

	delete(pr.memoryStore, pet.ID)
	return nil
}

func (pr *PetRepository) FindPetByStatus(
	ctx context.Context,
	req struct {
		Status []string `form:"status"`
	},
) ([]models.Pet, error) {
	// valid status
	for _, s := range req.Status {
		switch s {
		case "available", "pending", "sold":
		default:
			return nil, web.Error(http.StatusBadRequest, "Invalid status value")
		}
	}

	var findInArray = func(status string) bool {
		if 0 == len(req.Status) {
			return true
		}

		for _, s := range req.Status {
			if s == status {
				return true
			}
		}
		return false
	}

	var pets = make([]models.Pet, 0, len(pr.memoryStore))
	for _, pet := range pr.memoryStore {
		if findInArray(pet.Status) {
			pets = append(pets, pet)
		}
	}
	return pets, nil
}
