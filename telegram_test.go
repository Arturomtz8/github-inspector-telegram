package telegram

import (
	"testing"

	"github.com/Arturomtz8/github-inspector/pkg/github"
	"github.com/stretchr/testify/require"
)

func TestExtractParams(t *testing.T) {
	type input struct {
		s string
	}
	type expected struct {
		repo   string
		lang   string
		author string
	}

	var tests = []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "only repo",
			input: input{
				s: "/search dblab",
			},
			expected: expected{
				repo: "dblab",
			},
		},
		{
			name: "providing a lang",
			input: input{
				s: "/search dblab lang:go",
			},
			expected: expected{
				repo: "dblab",
				lang: "go",
			},
		},
		{
			name: "providing an author",
			input: input{
				s: "/search dblab author:danvergara",
			},
			expected: expected{
				repo:   "dblab",
				author: "danvergara",
			},
		},
		{
			name: "providing lang first",
			input: input{
				s: "/search dblab lang:go author:danvergara",
			},
			expected: expected{
				repo:   "dblab",
				author: "danvergara",
				lang:   "go",
			},
		},
		{
			name: "providing author first",
			input: input{
				s: "/search dblab author:danvergara lang:go",
			},
			expected: expected{
				repo:   "dblab",
				author: "danvergara",
				lang:   "go",
			},
		},
		{
			name: "providing an repo and author with dashes in the name",
			input: input{
				s: "/search go-swagger author:go-swagger lang:go",
			},
			expected: expected{
				repo:   "go-swagger",
				author: "go-swagger",
				lang:   "go",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, lang, author, err := ExtractParams(tc.input.s)
			require.NoError(t, err)
			require.Equal(t, tc.expected.repo, repo)
			require.Equal(t, tc.expected.lang, lang)
			require.Equal(t, tc.expected.author, author)
		})
	}
}

func TestParseRepoTemplate(t *testing.T) {
	var repo = &github.RepoTrending{
		FullName:    "danvergara/dblab",
		Description: "The database client every command line junkie deserves.",
		Owner: github.Owner{
			Login: "danvergara",
		},
		Language:        "Go",
		StargazersCount: 700,
	}

	s, err := parseRepoToTemplate(repo)
	t.Logf("repo %s", s)
	require.NoError(t, err)
	require.NotEmpty(t, s)
}
