import { test, type ApiHelpers } from './fixtures';
import { EntityPage, SearchPage } from '../pages';

// Owned items (TKT-QO14GB): a step is shown as part of the plan that owns it.
// Opening the step lands on the plan, anchored at the step, and a search hit
// on the step says which plan it is in and opens there.

test.describe('Owned items', () => {
  async function seed(api: ApiHelpers) {
    const plan = await api.createEntity('plans', { properties: { title: 'Launch plan' } });
    const step = await api.createEntity('steps', {
      properties: { title: 'Book the zeppelin hangar', assignee: 'Rowdy' },
    });
    await api.createRelation('plans', plan.id, 'has_step', step.id);
    return { planId: plan.id, stepId: step.id };
  }

  test("opening an owned entity shows its owner's page, anchored at it", async ({ appPage, api }) => {
    const { planId, stepId } = await seed(api);
    const entity = new EntityPage(appPage);

    await entity.navigateTo(`/entity/step/${stepId}`);

    await entity.expectShownAsPartOf('plan', planId, stepId);
    await entity.expectRelatedRowText(stepId, 'Book the zeppelin hangar');
    await entity.expectRelatedRowText(stepId, 'Rowdy');
  });

  test('a search hit names its owner and opens there', async ({ appPage, api }) => {
    const { planId, stepId } = await seed(api);
    const search = new SearchPage(appPage);

    await search.navigateToSearchWithQuery('zeppelin');
    await search.expectResultContains('in Launch plan');
    await search.focusFirstResult();
    await search.openSelectedResult();

    await new EntityPage(appPage).expectShownAsPartOf('plan', planId, stepId);
  });
});
