package model

type CardMachine struct {
	UUID              string //generate in code
	PersonNameStorage string `json:"person_name_storage" binding:"required"`
	StorageName       string `json:"storage_name" binding:"required"`
	PersonCpf         string `json:"person_cpf" binding:"required,min=11,max=11"`
	City              string `json:"city" binding:"required"`
	State             string `json:"state" binding:"required"`
	SerialNumber      string //generate in code
}

