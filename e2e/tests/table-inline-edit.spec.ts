import { test, type ApiHelpers } from './fixtures';
import { EntityPage } from '../pages';

// TKT-4Q2EIVZ: editable table cells and section totals on the entity detail
// page. The fixture's `invoice` view (DATA_ENTRY_YAML in fixtures.ts) shows
// the invoice's charges in a table whose description and qty columns are
// `render: input` and whose unit column is plain. Under the table a totals
// line reads `total_qty`, a roll-up summing the charges' qty.

const SECTION = 'Charges';
const TOTAL = 'Total quantity';

test.describe('Table sections with editable cells (TKT-4Q2EIVZ)', () => {
  async function seed(api: ApiHelpers) {
    const invoice = await api.createEntity('invoices', { properties: { title: 'March invoice' } });
    const widgets = await api.createEntity('charges', {
      properties: { description: 'Widgets', qty: 3, unit: 'pcs' },
    });
    const labour = await api.createEntity('charges', {
      properties: { description: 'Labour', qty: 2, unit: 'h' },
    });
    await api.createRelation('invoices', invoice.id, 'has_charge', widgets.id);
    await api.createRelation('invoices', invoice.id, 'has_charge', labour.id);
    return { invoiceId: invoice.id, widgetsId: widgets.id, labourId: labour.id };
  }

  test('input columns are editable and a plain column is not', async ({ appPage, api }) => {
    const { invoiceId, widgetsId } = await seed(api);
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('invoice', invoiceId);

    await entity.expectTableCellEditable(SECTION, widgetsId, 'description');
    await entity.expectTableCellEditable(SECTION, widgetsId, 'qty');
    await entity.expectTableCellReadOnly(SECTION, widgetsId, 'unit');
    await entity.expectTableCellText(SECTION, widgetsId, 'unit', 'pcs');
  });

  test('an edited cell is saved', async ({ appPage, api }) => {
    const { invoiceId, widgetsId } = await seed(api);
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('invoice', invoiceId);

    await entity.editTableCell(SECTION, widgetsId, 'description', 'Blue widgets');

    await appPage.reload();
    await entity.expectTableCellText(SECTION, widgetsId, 'description', 'Blue widgets');
    await entity.expectTableCellText(SECTION, widgetsId, 'unit', 'pcs');
  });

  test('the totals line follows an edit without a reload', async ({ appPage, api }) => {
    const { invoiceId, widgetsId } = await seed(api);
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('invoice', invoiceId);

    await entity.expectSectionTotal(SECTION, TOTAL, '5');

    await entity.editTableCell(SECTION, widgetsId, 'qty', '7');

    await entity.expectSectionTotal(SECTION, TOTAL, '9');

    await appPage.reload();
    await entity.expectTableCellText(SECTION, widgetsId, 'qty', '7');
    await entity.expectSectionTotal(SECTION, TOTAL, '9');
  });
});
