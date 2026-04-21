// Package report contains helpers to build prompts used to generate reports.
package report

import (
	"fmt"
	"strings"
)

// BuildPrompt builds the prompt text for the given achievements data and report type.
// reportType accepted values (case-insensitive): "executive", "technical", "timeline", "github".
func BuildPrompt(achievementsData, reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "technical", "tech":
		return buildTechnicalPrompt(achievementsData)
	case "timeline", "time":
		return buildTimelinePrompt(achievementsData)
	case "github", "git":
		return buildGitHubPrompt(achievementsData)
	default:
		return buildExecutivePrompt(achievementsData)
	}
}

func buildExecutivePrompt(data string) string {
	return fmt.Sprintf(`Você é um especialista sênior em RH e documentação profissional.

Analise as conquistas e crie um relatório executivo CONCISO seguindo esta estrutura:

# RESUMO EXECUTIVO
[2-3 frases impactantes com números]

# PRINCIPAIS REALIZAÇÕES
[4-6 conquistas mais importantes com impacto quantificado]

# COMPETÊNCIAS TÉCNICAS
[Linguagens, frameworks, ferramentas organizados]

# IMPACTO E VALOR
[3-4 frases sobre impacto real e valor agregado]

# PERFIL PROFISSIONAL
[2-3 frases caracterizando nível e expertise]

DADOS:
%s

IMPORTANTE: Seja específico, use números, foque em resultados.`, data)
}

func buildTechnicalPrompt(data string) string {
	return fmt.Sprintf(`Você é um arquiteto de software sênior.

Analise as contribuições técnicas e crie um relatório seguindo esta estrutura:

# VISÃO GERAL TÉCNICA
[Resumo das contribuições técnicas]

# ANÁLISE POR CATEGORIA
[Quantidade, complexidade e impacto por categoria]

# STACK TECNOLÓGICO
[Organize por Backend, Frontend, Infraestrutura, Banco de Dados]

# PADRÕES E BOAS PRÁTICAS
[Identifique padrões de arquitetura e qualidade de código]

# NÍVEL TÉCNICO
[Avalie complexidade e capacidade de resolver problemas]

DADOS:
%s

IMPORTANTE: Use terminologia técnica precisa, avalie profundidade.`, data)
}

func buildTimelinePrompt(data string) string {
	return fmt.Sprintf(`Você é um analista de desenvolvimento de carreira.

Analise a linha do tempo e crie uma narrativa de evolução:

# JORNADA PROFISSIONAL
[Introdução contextualizando a trajetória]

# LINHA DO TEMPO
[Organize cronologicamente destacando marcos]

# MARCOS IMPORTANTES
[3-5 momentos decisivos e seu impacto]

# EVOLUÇÃO DE COMPETÊNCIAS
[Do início ao estágio atual]

# PERSPECTIVAS
[Síntese do crescimento e potencial futuro]

DADOS:
%s

IMPORTANTE: Crie narrativa coesa mostrando progressão clara.`, data)
}

func buildGitHubPrompt(data string) string {
	return fmt.Sprintf(`Você é um especialista em análise de contribuições open source.

Analise as contribuições do GitHub:

# RESUMO DA ATIVIDADE
[Volume, frequência, diversidade]

# ANÁLISE DE CONTRIBUIÇÕES
[Commits, Pull Requests, Issues, Repositórios]

# QUALIDADE DAS CONTRIBUIÇÕES
[Consistência, complexidade técnica, impacto]

# PERFIL DO DESENVOLVEDOR
[Estilo, áreas de interesse, maturidade]

# DESTAQUES
[Principais conquistas e recomendações]

DADOS:
%s

IMPORTANTE: Qualidade > Quantidade, seja construtivo.`, data)
}
