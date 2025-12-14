package blockchain

import (
	"fmt"
	"sort"
)

// GetLatestIndexByKeyID returns the latest committed LMS index for each key_id
// Returns a map: keyID -> latest LMS index
func (v *VerusClient) GetLatestIndexByKeyID(identityName string) (map[string]string, error) {
	commits, err := v.QueryAttestationCommits(identityName, "")
	if err != nil {
		return nil, err
	}

	// Group by key_id and find latest by block height
	latestByKey := make(map[string]*AttestationCommit)
	for _, commit := range commits {
		if existing, ok := latestByKey[commit.KeyID]; !ok || commit.BlockHeight > existing.BlockHeight {
			latestByKey[commit.KeyID] = commit
		}
	}

	// Extract just the indices
	result := make(map[string]string)
	for keyID, commit := range latestByKey {
		result[keyID] = commit.LMSIndex
	}

	return result, nil
}

// GetLatestIndexForKey returns the latest committed LMS index for a specific key_id
func (v *VerusClient) GetLatestIndexForKey(identityName, keyID string) (string, error) {
	commits, err := v.QueryAttestationCommits(identityName, keyID)
	if err != nil {
		return "", err
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found for key_id: %s", keyID)
	}

	// Find latest by block height
	latest := commits[0]
	for _, commit := range commits[1:] {
		if commit.BlockHeight > latest.BlockHeight {
			latest = commit
		}
	}

	return latest.LMSIndex, nil
}

// GetAllKeyIDs returns all key_ids that have committed indices
func (v *VerusClient) GetAllKeyIDs(identityName string) ([]string, error) {
	identity, err := v.GetIdentity(identityName)
	if err != nil {
		return nil, err
	}

	if identity.Identity.ContentMultiMap == nil {
		return []string{}, nil
	}

	var keyIDs []string
	for keyID := range identity.Identity.ContentMultiMap {
		keyIDs = append(keyIDs, keyID)
	}

	sort.Strings(keyIDs)
	return keyIDs, nil
}

