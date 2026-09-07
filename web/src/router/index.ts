import { createRouter, createWebHistory } from 'vue-router'

/**
 * Routes follow the demo walkthrough in order: import, review, catalog, compose, draft.
 * The nav bar renders them in the same sequence.
 */
export const routes = [
  { path: '/', redirect: '/import' },
  {
    path: '/import',
    name: 'import',
    component: () => import('../pages/SourceImport.vue'),
    meta: { title: 'Source Import' },
  },
  {
    path: '/candidates',
    name: 'candidates',
    component: () => import('../pages/ExtractedCapabilities.vue'),
    meta: { title: 'Extracted Capabilities' },
  },
  {
    path: '/catalog',
    name: 'catalog',
    component: () => import('../pages/CapabilityCatalog.vue'),
    meta: { title: 'Capability Catalog' },
  },
  {
    path: '/compose',
    name: 'compose',
    component: () => import('../pages/CreateBusinessAgent.vue'),
    meta: { title: 'Create Business Agent' },
  },
  {
    path: '/drafts/:id?',
    name: 'drafts',
    component: () => import('../pages/BusinessAgentDraft.vue'),
    meta: { title: 'Business Agent Draft' },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
