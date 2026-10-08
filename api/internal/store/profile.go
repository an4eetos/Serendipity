package store

import (
	"context"
)

type Category struct {
	Name  string `json:"name"`
	Books int    `json:"books"`
}

type PublicBook struct {
	Work     Work    `json:"work"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	// PublicQuotes counts quotes from this book shown on the profile.
	PublicQuotes int `json:"public_quotes"`
}

type PublicQuote struct {
	Quote
	Conversations []Conversation `json:"conversations"`
}

type PublicProfile struct {
	Profile    Profile       `json:"profile"`
	Categories []Category    `json:"categories"`
	Books      []PublicBook  `json:"books"`
	Quotes     []PublicQuote `json:"quotes"`
}

// GetPublicProfile returns what anyone may see of a profile: its books,
// categories and public quotes with their conversations. Uploaded files are
// never included.
func (s *Store) GetPublicProfile(ctx context.Context, p Profile) (PublicProfile, error) {
	out := PublicProfile{Profile: p, Categories: []Category{}, Books: []PublicBook{}, Quotes: []PublicQuote{}}

	rows, err := s.pool.Query(ctx, `
		select `+workCols+`, li.status, li.progress,
			(select count(*) from quotes q where q.user_id = li.user_id and q.work_id = li.work_id and q.visibility = 'public')
		from library_items li join works w on w.id = li.work_id
		where li.user_id = $1 order by li.updated_at desc`, p.ID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var b PublicBook
		dest := append(scanWorkInto(&b.Work), &b.Status, &b.Progress, &b.PublicQuotes)
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return out, err
		}
		out.Books = append(out.Books, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = s.pool.Query(ctx, `
		select subject, count(*) from (
			select distinct li.work_id, unnest(w.subjects) as subject
			from library_items li join works w on w.id = li.work_id
			where li.user_id = $1
		) s group by subject order by count(*) desc, subject limit 12`, p.ID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Name, &c.Books); err != nil {
			rows.Close()
			return out, err
		}
		out.Categories = append(out.Categories, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = s.pool.Query(ctx, `select `+quoteCols+quoteFrom+`
		where q.user_id = $1 and q.visibility = 'public' order by q.created_at desc limit 100`, p.ID)
	if err != nil {
		return out, err
	}
	var quoteIDs []string
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Quotes = append(out.Quotes, PublicQuote{Quote: q, Conversations: []Conversation{}})
		quoteIDs = append(quoteIDs, q.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(quoteIDs) == 0 {
		return out, nil
	}

	convs, err := s.conversations(ctx, `c.user_id = $1 and c.quote_id = any($2::uuid[])`, p.ID, quoteIDs)
	if err != nil {
		return out, err
	}
	byQuote := map[string]*PublicQuote{}
	for i := range out.Quotes {
		byQuote[out.Quotes[i].ID] = &out.Quotes[i]
	}
	for _, c := range convs {
		// Skip conversations that never got a reply (e.g. a failed request).
		if len(c.Messages) == 0 {
			continue
		}
		if q := byQuote[c.QuoteID]; q != nil {
			q.Conversations = append(q.Conversations, c)
		}
	}
	return out, nil
}
