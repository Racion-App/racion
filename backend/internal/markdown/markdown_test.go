package markdown

import "testing"

func TestRender(t *testing.T) {
	cases := map[string]string{
		"Просто текст":                   "<p>Просто текст</p>",
		"**жирно** и *курсив* и ~~нет~~": "<p><strong>жирно</strong> и <em>курсив</em> и <s>нет</s></p>",
		"- один\n- два":                  "<ul><li>один</li><li>два</li></ul>",
		"1. раз\n2. два":                 "<ol><li>раз</li><li>два</li></ol>",
		"> цитата":                       "<blockquote>цитата</blockquote>",
		"[сайт](https://a.b/c)":          `<p><a href="https://a.b/c" rel="nofollow noopener" target="_blank">сайт</a></p>`,
		"см https://a.b":                 `<p>см <a href="https://a.b" rel="nofollow noopener" target="_blank">https://a.b</a></p>`,
		"<script>alert(1)</script>":      "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>",
		"[x](javascript:alert(1))":       "<p>[x](javascript:alert(1))</p>",
		"строка\nещё\n\nабзац":           "<p>строка<br>ещё</p><p>абзац</p>",
		"код `a<b`":                      "<p>код <code>a&lt;b</code></p>",
		"снежинка 2*3*4":                 "<p>снежинка 2*3*4</p>",
	}
	for in, want := range cases {
		if got := Render(in); got != want {
			t.Errorf("%q:\n got  %s\n want %s", in, got, want)
		}
	}
}
