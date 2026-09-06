import { defineConfig } from 'vitepress'

const description = 'Vibeporter preserves engineering context locally and makes it portable across AI agents, developers, teams, and projects.'

export default defineConfig({
  title: "vibeporter",
  description,
  appearance: 'dark',
  head: [
    ['link', { rel: 'icon', href: '/favicon.svg' }],
    ['meta', { name: 'description', content: description }],
    ['meta', { property: 'og:title', content: 'Vibeporter' }],
    ['meta', { property: 'og:description', content: description }],
    ['meta', { property: 'og:image', content: 'https://vibeporter.pages.dev/social.png' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:title', content: 'Vibeporter' }],
    ['meta', { name: 'twitter:description', content: description }],
    ['meta', { name: 'twitter:image', content: 'https://vibeporter.pages.dev/social.png' }],
  ],
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Scenarios', link: '/scenarios' },
      { text: 'Context', link: '/context' },
      { text: 'Teams', link: '/teams' },
      { text: 'GitHub', link: 'https://github.com/iamvxrn/vibeporter' }
    ],
    search: {
      provider: 'local'
    },
    sidebar: [
      {
        text: 'START',
        items: [
          { text: 'Overview', link: '/overview' },
          { text: 'Install', link: '/install' },
          { text: 'Quickstart', link: '/quickstart' },
          { text: 'Scenarios', link: '/scenarios' },
          { text: 'Context model', link: '/context' },
          { text: 'For teams', link: '/teams' },
          { text: 'CLI Reference', link: '/cli' },
          { text: 'Changelog', link: '/changelog' },
        ]
      },
      {
        text: 'INTEGRATIONS',
        items: [
          { text: 'Integrations', link: '/integrations' },
          { text: 'Config Porting', link: '/config-porting' },
        ]
      },
      {
        text: 'ADAPTERS',
        items: [
          { text: 'Claude Code', link: '/claudecode' },
          { text: 'OpenCode', link: '/opencode' },
          { text: 'Gemini CLI', link: '/gemini' },
          { text: 'Kimi Code', link: '/kimicode' },
          { text: 'DeepSeek Harness', link: '/dsh' },
          { text: 'Cursor', link: '/cursor' },
        ]
      },
      {
        text: 'BUSINESS',
        items: [
          { text: 'Business', link: '/business' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/iamvxrn/vibeporter' }
    ]
  }
})
