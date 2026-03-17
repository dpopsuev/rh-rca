package rp

// FetchAndSave runs pre-investigation: fetch envelope from fetcher and save to store.
func FetchAndSave(f EnvelopeFetcher, s EnvelopeStore, launchID int) error {
	env, err := f.Fetch(launchID)
	if err != nil {
		return err
	}
	return s.Save(launchID, env)
}
