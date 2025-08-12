package tcabcireadgoclient

func StringOR(values ...string) string {
	for i := 0; i < len(values); i++ {
		if values[i] != "" {
			return values[i]
		}
	}

	return ""
}
