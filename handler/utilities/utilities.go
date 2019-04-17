package utilities

import "fmt"

func ValidateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("please enter the username")
	}
	return nil
}

func ValidatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return fmt.Errorf("please enter the password twice")
	}
	if !(pOne == pTwo) {
		return fmt.Errorf("the two passwords entered were not the same")
	}
	return nil
}
