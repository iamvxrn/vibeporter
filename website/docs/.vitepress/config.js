import { defineConfig } from 'vitepress'

export default defineConfig({
  title: "vibeporter",
  description: "Migrate chat histories and configs between AI coding agents.",
  appearance: 'dark',
  head: [['link', { rel: 'icon', href: '/favicon.svg' }]],
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Home', link: '/' },
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
          { text: 'CLI Reference', link: '/cli' },
          { text: 'Changelog', link: '/changelog' },
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
        ]
      },
      {
        text: 'WORKFLOWS',
        items: [
          { text: 'Config Porting', link: '/config-porting' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/iamvxrn/vibeporter' }
    ]
  }
})
