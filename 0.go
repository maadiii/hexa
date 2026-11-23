package hexa

import "github.com/go-playground/validator/v10"

type Config struct {
	DevMode bool
	Port    int
	Name    string
	CORS    []string
}

type Validator struct {
	validator *validator.Validate
}

func newValidator() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

func (v *Validator) Validate(i any) (err error) {
	err = v.validator.Struct(i)

	return
}
