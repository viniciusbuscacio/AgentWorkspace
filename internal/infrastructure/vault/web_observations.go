package vault

import (
	"database/sql"
	"strings"

	"aw/internal/domain"
)

func (v *Vault) SaveWebObservation(obs domain.WebObservation) (domain.WebObservation, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.WebObservation{}, errLocked
	}
	if err := v.clearWebObservationsForSiteLocked(obs.ChatID, obs.Browser, obs.SiteKey); err != nil {
		return domain.WebObservation{}, err
	}
	if strings.TrimSpace(obs.CapturedAt) == "" {
		obs.CapturedAt = nowString()
	}
	_, err := v.db.Exec(
		`INSERT INTO web_observations
			(id, chat_id, run_id, browser, tab_id, site_key, url, title, captured_at, payload_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID,
		obs.ChatID,
		obs.RunID,
		obs.Browser,
		obs.TabID,
		obs.SiteKey,
		obs.URL,
		obs.Title,
		obs.CapturedAt,
		obs.PayloadJSON,
	)
	if err != nil {
		return domain.WebObservation{}, err
	}
	return obs, nil
}

func (v *Vault) LatestWebObservation(chatID, browser, siteKey string) (domain.WebObservation, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.WebObservation{}, false, errLocked
	}
	clauses := []string{"chat_id = ?"}
	args := []any{strings.TrimSpace(chatID)}
	if strings.TrimSpace(browser) != "" {
		clauses = append(clauses, "browser = ?")
		args = append(args, strings.TrimSpace(browser))
	}
	if strings.TrimSpace(siteKey) != "" {
		clauses = append(clauses, "site_key = ?")
		args = append(args, strings.TrimSpace(siteKey))
	}
	query := `SELECT id, chat_id, run_id, browser, tab_id, site_key, url, title, captured_at, payload_json
		FROM web_observations
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY captured_at DESC
		LIMIT 1`
	var obs domain.WebObservation
	err := v.db.QueryRow(query, args...).Scan(
		&obs.ID,
		&obs.ChatID,
		&obs.RunID,
		&obs.Browser,
		&obs.TabID,
		&obs.SiteKey,
		&obs.URL,
		&obs.Title,
		&obs.CapturedAt,
		&obs.PayloadJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.WebObservation{}, false, nil
		}
		return domain.WebObservation{}, false, err
	}
	return obs, true, nil
}

func (v *Vault) ClearWebObservations() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	return v.clearWebObservationsLocked()
}

func (v *Vault) clearWebObservationsLocked() error {
	_, err := v.db.Exec(`DELETE FROM web_observations`)
	return err
}

func (v *Vault) clearWebObservationsForSiteLocked(chatID, browser, siteKey string) error {
	_, err := v.db.Exec(
		`DELETE FROM web_observations WHERE chat_id = ? AND browser = ? AND site_key = ?`,
		strings.TrimSpace(chatID),
		strings.TrimSpace(browser),
		strings.TrimSpace(siteKey),
	)
	return err
}
