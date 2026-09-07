import { useEffect, useId } from 'react';

type ApiDocsPanelProps = {
  /** Static asset prefix, e.g. `/admin` */
  assetBase: string;
  /** OpenAPI JSON URL (admin-authenticated), e.g. `/admin/api/admin/docs/openapi.json` */
  specUrl: string;
};

declare global {
  interface Window {
    SwaggerUIBundle?: {
      (config: Record<string, unknown>): unknown;
      presets: { apis: unknown; SwaggerUIStandalonePreset: unknown };
    };
    ui?: { destroy?: () => void };
  }
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[src="${src}"]`);
    if (existing) {
      resolve();
      return;
    }
    const script = document.createElement('script');
    script.src = src;
    script.crossOrigin = 'anonymous';
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`failed to load ${src}`));
    document.body.appendChild(script);
  });
}

function loadStylesheet(href: string): () => void {
  const existing = document.querySelector(`link[href="${href}"]`);
  if (existing) {
    return () => {};
  }
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = href;
  document.head.appendChild(link);
  return () => link.remove();
}

export function ApiDocsPanel({ assetBase, specUrl }: ApiDocsPanelProps) {
  const domId = useId().replace(/:/g, '');

  useEffect(() => {
    const removeCss = loadStylesheet(`${assetBase}/swagger-ui.css`);
    let destroyed = false;

    loadScript(`${assetBase}/swagger-ui-bundle.js`)
      .then(() => {
        if (destroyed || !window.SwaggerUIBundle) {
          return;
        }
        window.ui = window.SwaggerUIBundle({
          url: specUrl,
          dom_id: `#${domId}`,
          deepLinking: true,
          presets: [
            window.SwaggerUIBundle.presets.apis,
            window.SwaggerUIBundle.presets.SwaggerUIStandalonePreset,
          ],
          layout: 'BaseLayout',
          requestInterceptor: (req: { credentials?: string }) => {
            req.credentials = 'same-origin';
            return req;
          },
        }) as { destroy?: () => void };
      })
      .catch(() => {
        /* swallow — empty panel */
      });

    return () => {
      destroyed = true;
      removeCss();
      window.ui?.destroy?.();
      window.ui = undefined;
    };
  }, [assetBase, specUrl, domId]);

  return <div id={domId} className="h-full min-h-[480px] w-full overflow-auto" />;
}
