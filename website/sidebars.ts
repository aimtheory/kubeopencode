import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'getting-started',
    'setting-up-agent',
    'features',
    'architecture',
    'security',
    {
      type: 'category',
      label: 'Operations',
      items: [
        'operations/troubleshooting',
        'operations/releasing',
        'operations/upgrading',
      ],
    },
    'roadmap',
  ],
};

export default sidebars;
