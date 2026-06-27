import { test } from './fixtures';
import { EntityPage } from '../pages';
import { SEED } from './fixtures';

// Regression test for issue #997.
//
// Navigating away from an /entity/:type/:id detail page that renders a
// configured `display: list` relation section (whose rows mount a
// SectionEditForm with an AutoSaveIndicator) threw an uncaught Vue-internal
// error during the route-driven unmount:
//
//   TypeError: Cannot read properties of null (reading 'type')   (dev build)
//   TypeError: Cannot destructure property 'bum' of 'e' as it is null  (prod)
//
// The crash is swallowed by Vue and only surfaces through the SPA's global
// `app.config.errorHandler`. The `appPage` fixture captures those
// `[vue-error]` console lines and fails the test in afterEach (see
// fixtures.ts), so the actual assertion lives there — every navigation in
// the suite is now an unmount-error probe. This spec's job is to MOUNT the
// crashing shape and then navigate away, so the fixture guard has something
// to catch.
//
// The fixture's `task` view (see DATA_ENTRY_YAML in fixtures.ts) has an
// "Implements" `display: list` section; TASK-001 implements FEAT-001 in the
// seed, so the section renders one populated row — the exact shape that
// crashed. The view lives on `task` (not `feature`) to avoid perturbing the
// default feature rendering other specs assert against.

test.describe('Entity detail — navigate away from a populated list section (issue #997)', () => {
  test('does not throw a Vue unmount error when leaving a list-section page', async ({ appPage }) => {
    const entity = new EntityPage(appPage);

    // TASK-001 implements FEAT-001 in the seed, so the task view's
    // "Implements" display:list section renders one populated row with an
    // inline-edit form.
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // The list row (and its inline AutoSaveIndicator) must actually be
    // mounted, or navigating away would not exercise the crash path and the
    // test would pass vacuously.
    await entity.expectListSectionRowMounted();

    // Navigate away via an in-SPA router link — this unmounts the
    // EntityDetail subtree (list rows + their indicators) on the client.
    // Pre-fix, this is where the crash fired; the appPage fixture's
    // vue-error guard fails the test in afterEach if it recurs. A full page
    // reload would NOT exercise the unmount path.
    await entity.navigateAwayViaRouter();
    await entity.waitForSpinnerToDisappear();
  });
});
