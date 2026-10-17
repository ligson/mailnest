package storage

import (
	"context"
	"sort"
	"strings"
)

type ContactResolution struct {
	Items             []Contact
	AmbiguousPrefixes []string
}

// ResolveContacts 只读取当前页需要的地址；前缀歧义在完整通讯录上判断，不能用局部缓存猜测唯一性。
func (s *Store) ResolveContacts(ctx context.Context, userID int64, keys []string) (ContactResolution, error) {
	exact := map[string]bool{}
	prefixes := map[string]bool{}
	for _, key := range keys {
		key = normalizeEmailKey(key)
		if key == "" {
			continue
		}
		exact[key] = true
		prefixes[strings.SplitN(key, "@", 2)[0]] = true
	}
	if len(exact) == 0 {
		return ContactResolution{Items: []Contact{}, AmbiguousPrefixes: []string{}}, nil
	}
	normalized := make([]string, 0, len(exact))
	for key := range exact {
		normalized = append(normalized, key)
	}
	sort.Strings(normalized)
	value, err := s.mailReads.get(ctx, s.readKey("contacts.resolve", contactResolveQuery{userID, normalized}), func(work context.Context) (any, error) {
		where := []string{}
		args := []any{userID}
		escape := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
		for _, key := range normalized {
			where = append(where, "email_key=?")
			args = append(args, key)
		}
		ordered := []string{}
		for prefix := range prefixes {
			ordered = append(ordered, prefix)
		}
		sort.Strings(ordered)
		for _, prefix := range ordered {
			where = append(where, "email_key LIKE ? ESCAPE '!'")
			args = append(args, escape.Replace(prefix)+"@%")
		}
		rows, e := s.db.QueryContext(work, `SELECT id,user_id,email,email_key,display_name,nickname,phone,company,notes,source,first_seen_at,last_seen_at,created_at,updated_at FROM contacts WHERE user_id=? AND (`+strings.Join(where, " OR ")+`) ORDER BY id`, args...)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		all := []Contact{}
		counts := map[string]int{}
		for rows.Next() {
			contact, e := scanContact(rows)
			if e != nil {
				return nil, e
			}
			all = append(all, contact)
			counts[strings.SplitN(contact.EmailKey, "@", 2)[0]]++
		}
		if e = rows.Err(); e != nil {
			return nil, e
		}
		result := ContactResolution{Items: []Contact{}, AmbiguousPrefixes: []string{}}
		for _, prefix := range ordered {
			if counts[prefix] > 1 {
				result.AmbiguousPrefixes = append(result.AmbiguousPrefixes, prefix)
			}
		}
		for _, contact := range all {
			prefix := strings.SplitN(contact.EmailKey, "@", 2)[0]
			if exact[contact.EmailKey] || counts[prefix] == 1 {
				result.Items = append(result.Items, contact)
			}
		}
		return result, nil
	})
	if err != nil {
		return ContactResolution{}, err
	}
	result := value.(ContactResolution)
	result.Items = append([]Contact{}, result.Items...)
	result.AmbiguousPrefixes = append([]string{}, result.AmbiguousPrefixes...)
	return result, nil
}
