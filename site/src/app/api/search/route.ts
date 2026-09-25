import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

// The static export has no server to run a search endpoint on, so the index is built once at
// build time and the client searches it in the browser. RootProvider is configured to match.
export const revalidate = false;
export const dynamic = 'force-static';

export const { staticGET: GET } = createFromSource(source);
