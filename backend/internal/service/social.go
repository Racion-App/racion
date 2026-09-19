package service

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"racion/internal/domain"
	"racion/internal/markdown"
	"racion/internal/planner"
)

// Лайки, избранное и комментарии к рецептам — базовым и своим открытым. Ник обязателен только для
// комментариев: без него нечем подписать.

type SocialRepo interface {
	Like(ctx context.Context, userID, recipeID string) error
	Unlike(ctx context.Context, userID, recipeID string) error
	Favorite(ctx context.Context, userID, recipeID string) error
	Unfavorite(ctx context.Context, userID, recipeID string) error
	Favorites(ctx context.Context, userID string) ([]string, error)
	Stats(ctx context.Context, recipeID, userID string) (domain.RecipeStats, error)
	LikeCounts(ctx context.Context) (map[string]int, error)
	Comments(ctx context.Context, recipeID string, limit int) ([]domain.Comment, error)
	AddComment(ctx context.Context, recipeID, userID, body, image string) (int64, error)
	DeleteComment(ctx context.Context, id int64, userID string) error
	CountRecent(ctx context.Context, userID string) (int, error)
	SetFeedback(ctx context.Context, userID, recipeID string, liked bool) error
	Feedback(ctx context.Context, userID string) (liked map[string]bool, meh map[string]int, err error)
}

type Social struct {
	repo      SocialRepo
	recipes   *Recipes
	mediaOwns func(url string) bool
}

const (
	commentMax     = 1000
	commentPerHour = 30
	commentsShown  = 50
)

var nickRe = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)

// NormalizeNick приводит ник к виду a-z0-9_ длиной 3–24; пусто — снять ник.
func NormalizeNick(nick string) (string, error) {
	nick = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(nick, "@")))
	if nick == "" {
		return "", nil
	}
	if !nickRe.MatchString(nick) {
		return "", domain.Invalid("nick.invalid")
	}
	return nick, nil
}

// exists — рецепт есть и его можно обсуждать: базовый или свой (по id, ссылка общая).
func (s *Social) exists(ctx context.Context, recipeID string) error {
	if len(recipeID) > 80 {
		return domain.ErrNotFound
	}
	_, err := s.recipes.Find(ctx, recipeID)
	return err
}

func (s *Social) Like(ctx context.Context, userID, recipeID string, on bool) (domain.RecipeStats, error) {
	if err := s.exists(ctx, recipeID); err != nil {
		return domain.RecipeStats{}, err
	}
	var err error
	if on {
		err = s.repo.Like(ctx, userID, recipeID)
	} else {
		err = s.repo.Unlike(ctx, userID, recipeID)
	}
	if err != nil {
		return domain.RecipeStats{}, err
	}
	return s.repo.Stats(ctx, recipeID, userID)
}

func (s *Social) Favorite(ctx context.Context, userID, recipeID string, on bool) (domain.RecipeStats, error) {
	if err := s.exists(ctx, recipeID); err != nil {
		return domain.RecipeStats{}, err
	}
	var err error
	if on {
		err = s.repo.Favorite(ctx, userID, recipeID)
	} else {
		err = s.repo.Unfavorite(ctx, userID, recipeID)
	}
	if err != nil {
		return domain.RecipeStats{}, err
	}
	return s.repo.Stats(ctx, recipeID, userID)
}

// Feedback — «как было?» после ужина. Ошибка только если рецепта нет.
func (s *Social) Feedback(ctx context.Context, userID, recipeID string, liked bool) error {
	if _, err := s.recipes.Find(ctx, recipeID); err != nil {
		return err
	}
	return s.repo.SetFeedback(ctx, userID, recipeID, liked)
}

// Taste — что выучили по ответам: liked — бонус в подборе, meh — штраф после одного «не зашло»,
// banned — после двух блюдо не предлагается.
func (s *Social) Taste(ctx context.Context, userID string) (liked, meh, banned []string) {
	l, m, err := s.repo.Feedback(ctx, userID)
	if err != nil {
		return nil, nil, nil
	}
	for id, ok := range l {
		switch {
		case ok:
			liked = append(liked, id)
		case m[id] >= 2:
			banned = append(banned, id)
		default:
			meh = append(meh, id)
		}
	}
	return liked, meh, banned
}

// SetMedia подключает проверку ссылок на фото.
func (s *Social) SetMedia(owns func(url string) bool) { s.mediaOwns = owns }

// FavoriteIDs — избранное для планировщика; при сбое пусто.
func (s *Social) FavoriteIDs(ctx context.Context, userID string) []string {
	ids, _ := s.repo.Favorites(ctx, userID)
	return ids
}

// Favorites — избранные рецепты целиком (базовые и свои), в порядке добавления.
func (s *Social) Favorites(ctx context.Context, userID string) ([]planner.Recipe, error) {
	ids, err := s.repo.Favorites(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]planner.Recipe, 0, len(ids))
	for _, id := range ids {
		if rc, err := s.recipes.Find(ctx, id); err == nil {
			out = append(out, rc)
		}
	}
	return out, nil
}

// Stats — счётчики и отметки текущего пользователя (userID пустой у гостя).
func (s *Social) Stats(ctx context.Context, recipeID, userID string) domain.RecipeStats {
	st, _ := s.repo.Stats(ctx, recipeID, userID)
	return st
}

func (s *Social) LikeCounts(ctx context.Context) map[string]int {
	m, _ := s.repo.LikeCounts(ctx)
	return m
}

func (s *Social) Comments(ctx context.Context, recipeID, viewerID string) ([]domain.Comment, error) {
	if err := s.exists(ctx, recipeID); err != nil {
		return nil, err
	}
	list, err := s.repo.Comments(ctx, recipeID, commentsShown)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].HTML = markdown.Render(list[i].Body)
	}
	for i := range list {
		list[i].Mine = viewerID != "" && list[i].UserID == viewerID
		list[i].UserID = "" // чужие id наружу не отдаём
	}
	return list, nil
}

// AddComment — текст 1–500 знаков, не больше 30 в час; автору нужен ник.
func (s *Social) AddComment(ctx context.Context, user domain.User, recipeID, body, image string) (domain.Comment, error) {
	if user.Nick == "" {
		return domain.Comment{}, domain.Invalid("comment.nick")
	}
	body = strings.TrimSpace(body)
	if n := utf8.RuneCountInString(body); n == 0 || n > commentMax {
		return domain.Comment{}, domain.Invalid("comment.length")
	}
	if image != "" && (s.mediaOwns == nil || !s.mediaOwns(image)) {
		return domain.Comment{}, domain.Invalid("photo.bad")
	}
	if err := s.exists(ctx, recipeID); err != nil {
		return domain.Comment{}, err
	}
	if n, err := s.repo.CountRecent(ctx, user.ID); err == nil && n >= commentPerHour {
		return domain.Comment{}, domain.Invalid("comment.toomany")
	}
	id, err := s.repo.AddComment(ctx, recipeID, user.ID, body, image)
	if err != nil {
		return domain.Comment{}, err
	}
	return domain.Comment{ID: id, Nick: user.Nick, Name: user.Name, Body: body, HTML: markdown.Render(body), Image: image, Avatar: user.Avatar, Mine: true}, nil
}

func (s *Social) DeleteComment(ctx context.Context, userID string, id int64) error {
	return s.repo.DeleteComment(ctx, id, userID)
}
