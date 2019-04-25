package handler

import "fmt"

func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("please enter the username")
	}
	return nil
}

func validatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return fmt.Errorf("please enter the password twice")
	}
	if !(pOne == pTwo) {
		return fmt.Errorf("the two passwords entered were not the same")
	}
	return nil
}

func validateAPIKey(apiKey, secret string) error {
	if apiKey == "" || secret == "" {
		return fmt.Errorf("please enter both the api key and secret")
	}
	return nil
}

func firstExisting(or string, strings ...string) string {
	current := ""
	for _, s := range strings {
		if s == "" {
			continue
		}
		current = s
		break
	}
	if current == "" {
		return or
	}
	return current
}
