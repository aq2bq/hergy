// setup.js is the unit gate's preload (go run ./tools/check unit): a document
// and a window from happy-dom, so the WebUI's modules run under bun test as
// they do in a page, with no browser. A test lays out what the page would hold
// (the embedded data, the mount points) before it imports a screen's module.
import { GlobalRegistrator } from '@happy-dom/global-registrator';

GlobalRegistrator.register({ url: 'http://127.0.0.1/' });
