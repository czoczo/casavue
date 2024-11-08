import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { rehypeAccessibleEmojis } from 'rehype-accessible-emojis';

// https://astro.build/config
export default defineConfig({
	//site: 'https://casavue.app',
	site: 'https://czoczo.github.io',
	markdown: {
	  // Applied to .md and .mdx files
	  rehypePlugins: [rehypeAccessibleEmojis],
	},
	integrations: [
		starlight({
			title: 'CasaVue',
			customCss: [
				// Relative path to your custom CSS file
				'./src/styles/casavue.css',
			  ],
			logo: {
				src: './src/assets/logo.svg',
			  },
			social: {
				github: 'https://github.com/czoczo/casavue',
			},
			sidebar: [
				{
					label: 'About',
					autogenerate: { directory: 'about'},
				},
				{
					label: 'Deployment',
					autogenerate: { directory: 'deployment' },
				},
				{
					label: 'Configuration',
					autogenerate: { directory: 'configuration' },

					//items: [
					//	// Each item here is one entry in the navigation menu.
					//	{ label: 'Example Guide', link: '/guides/example/' },
					//],
				},
				{
					label: 'Development',
					autogenerate: { directory: 'development' },
				},
			],
		}),
	],
});
