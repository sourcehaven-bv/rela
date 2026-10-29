/**
 * Dutch for every string the library composes; see `useMessages.ts`.
 *
 * Typed as the full `Messages`, not a partial, so a string added to the
 * library fails the type check here until it has a Dutch version. That is the
 * library's key-parity check.
 *
 * Plain, informal-neutral Dutch: "je", not "u".
 */
import type { ChannelTextKey, ChromeTextKey, CountArgs, Messages } from './useMessages'

function counted({ count, noun, pluralNoun }: CountArgs): string {
  // The noun is the app's word, already in the app's language. Dutch plurals
  // are irregular, so without an explicit plural the singular is kept rather
  // than guessed with an English "s".
  return `${count} ${count === 1 ? noun : (pluralNoun ?? noun)}`
}

/** `a`, `a of b`, `a, b of c`. */
function ofList(items: string[]): string {
  if (items.length <= 1) return items.join('')
  return `${items.slice(0, -1).join(', ')} of ${items[items.length - 1]}`
}

const CHROME_TEXT_NL: Record<ChromeTextKey, string> = {
  sidebar: 'Zijbalk',
  mainNavigation: 'Hoofdmenu',
  collapseSidebar: 'Zijbalk inklappen',
  closeNavigation: 'Navigatie sluiten',
  filterNavigation: 'Navigatie filteren',
  filter: 'Filteren',
  otherGroup: 'Overig',
  colourTheme: 'Kleurenthema',
  themeSystem: 'Systeem',
  themeLight: 'Licht',
  themeDark: 'Donker',
  emptyValue: 'Leeg',
  helpDocs: 'Help',
  add: 'Toevoegen',
  cancel: 'Annuleren',
  save: 'Opslaan',
  edit: 'Bewerken',
  delete: 'Verwijderen',
  back: 'Terug',
  expand: 'Vergroten',
  previous: 'Vorige',
  next: 'Volgende',
  help: 'Help',
  more: 'meer',
  loading: 'Laden',
  working: 'Bezig',
  saving: 'Opslaan',
  saved: 'Opgeslagen',
  couldNotSave: 'Opslaan is mislukt',
  search: 'Zoeken',
  clearSearch: 'Zoekopdracht wissen',
  clearSelection: 'Selectie wissen',
  select: 'Selecteren',
  selectOptions: 'Opties selecteren',
  from: 'Van',
  to: 'Tot',
  name: 'Naam',
  unassigned: 'Niet toegewezen',
  assignedTo: 'Toegewezen aan',
  noMatchingPeople: 'Geen personen gevonden',
  due: 'Uiterlijk op',
  today: 'Vandaag',
  keyThen: 'dan',
  keyOr: 'of',
  openNavigation: 'Navigatie openen',
  moreOptions: 'Meer opties',
  star: 'Ster geven',
  like: 'Leuk vinden',
  share: 'Delen',
  copyLink: 'Link kopiëren',
  attachFile: 'Bestand bijvoegen',
  closePanel: 'Paneel sluiten',
  closeDialog: 'Venster sluiten',
  resizeSidebar: 'Breedte van de zijbalk aanpassen',
  resizePanel: 'Breedte van het paneel aanpassen',
  view: 'Weergave',
  documentActions: 'Documentacties',
  taskActions: 'Taakacties',
  pagination: 'Paginering',
  previousPage: 'Vorige pagina',
  nextPage: 'Volgende pagina',
  notifications: 'Meldingen',
  dismissMessage: 'Bericht sluiten',
  dismissNotification: 'Melding sluiten',
  commandPalette: 'Opdrachten',
  commandPlaceholder: 'Typ een opdracht of zoek',
  noMatchingCommands: 'Geen opdrachten gevonden',
  addSection: 'Sectie toevoegen',
  draggableCard: 'Versleepbare kaart',
  moveRowHint: 'Verplaats deze rij met de pijltjestoetsen omhoog en omlaag',
  subtasks: 'subtaken',
  continuesAfter: 'Loopt door op de volgende dagen',
  continuesBeforeAndAfter: 'Loopt door voor en na deze dag',
  continuedFromBefore: 'Begon op een eerdere dag',
  commentNoun: 'reactie',
  comments: 'reacties',
  commentAction: 'Reageren',
  addComment: 'Reactie toevoegen',
  addCommentLabel: 'Reactie toevoegen',
  addCommentPlaceholder: 'Reactie toevoegen...',
  replyPlaceholder: 'Antwoorden...',
  reply: 'Antwoorden',
  commentBody: 'Tekst van de reactie',
  commentAbout: 'Waar deze reactie over gaat',
  editComment: 'Reactie bewerken',
  postingComment: 'Reactie plaatsen',
  toPost: 'om te plaatsen',
  checking: 'Controleren...',
  allComments: 'Alle reacties',
  noComments: 'Nog geen reacties',
  startThread: 'Begin het gesprek hieronder.',
  closeComments: 'Reacties sluiten',
  detached: 'losgeraakt',
  detachedComment: 'Waar deze reactie naar verwijst, bestaat niet meer',
  resolve: 'Afhandelen',
  reopen: 'Heropenen',
  description: 'Beschrijving',
  attachments: 'Bijlagen',
  subtasksHeading: 'Subtaken',
  showEmptyFields: 'Lege velden tonen',
  download: 'Downloaden',
  fileKindSheet: 'REKENBLAD',
  fileKindImage: 'AFBEELDING',
  fileKindOther: 'BESTAND',
  formatting: 'Opmaak',
  table: 'Tabel',
  writeSomething: 'Schrijf iets...',
  docTitlePlaceholder: 'Geef je document een titel...',
  docBodyPlaceholder: 'Klik hier om te beginnen met schrijven',
  code: 'Code',
  copyCode: 'Code kopiëren',
  copied: 'Gekopieerd',
}

/** Toolbar names of the library's own editor commands, by command id. */
const EDITOR_COMMANDS_NL: Record<string, string> = {
  strong: 'Vet',
  emphasis: 'Cursief',
  strikethrough: 'Doorhalen',
  inlineCode: 'Code in de tekst',
  h1: 'Kop 1',
  h2: 'Kop 2',
  h3: 'Kop 3',
  bulletList: 'Lijst met opsommingstekens',
  orderedList: 'Genummerde lijst',
  taskList: 'Takenlijst',
  blockquote: 'Citaat',
  codeBlock: 'Codeblok',
  table: 'Tabel',
  addRowBefore: 'Rij erboven invoegen',
  addRowAfter: 'Rij eronder invoegen',
  addColBefore: 'Kolom links invoegen',
  addColAfter: 'Kolom rechts invoegen',
  deleteRow: 'Rij verwijderen',
  deleteColumn: 'Kolom verwijderen',
  deleteTable: 'Tabel verwijderen',
}

const CHANNEL_TEXT_NL: Record<ChannelTextKey, string> = {
  newMessage: 'Nieuw bericht',
  editMessage: 'Bericht bewerken',
  messageType: 'Soort bericht',
  cancelReply: 'Antwoord annuleren',
  required: 'Verplicht.',
  toSend: 'om te versturen',
  toSave: 'om op te slaan',
  send: 'Versturen',
  sending: 'Versturen…',
  save: 'Opslaan',
  saving: 'Opslaan…',
  cancel: 'Annuleren',
  typeGone: 'Deze soort bericht bestaat niet meer, dus het bericht kan niet worden bewerkt.',
  choose: 'Kiezen…',
  choosing: 'Kiezen…',
  change: 'Wijzigen…',
  replace: 'Vervangen…',
  clear: 'Wissen',
  closed: 'Gesloten',
  archived: 'Gearchiveerd',
  working: 'Bezig…',
  messages: 'Berichten',
  loadOlder: 'Oudere berichten laden',
  loading: 'Laden…',
  noMessages: 'Nog geen berichten.',
  jumpToLatest: 'Naar het nieuwste',
  closedNotice: 'Dit kanaal is gesloten. Je kunt het lezen, maar er kunnen geen berichten bij.',
  archivedNotice:
    'Dit kanaal is gearchiveerd. Je kunt het lezen, maar er kunnen geen berichten bij.',
  yes: 'Ja',
  no: 'Nee',
  edited: '(bewerkt)',
  reply: 'Antwoorden',
  edit: 'Bewerken',
  delete: 'Verwijderen',
  deleting: 'Verwijderen…',
  remove: 'Weghalen…',
  removing: 'Weghalen…',
  release: 'Vrijgeven',
  releasing: 'Vrijgeven…',
  showChannels: 'Kanalen tonen',
  filterOpen: 'Open',
  filterAll: 'Alle',
  newChannel: 'Nieuw kanaal',
  noChannels: 'Nog geen kanalen.',
  system: 'systeem',
  agent: 'agent',
  restrictedRef: 'Afgeschermde verwijzing',
  restrictedRefTitle: 'Je hebt geen toegang tot dit item',
  session: 'Sessie',
  sessionEnded: 'Beëindigd',
  sessionCompleted: 'Voltooid',
  sessionCancelled: 'Geannuleerd',
  sessionFailed: 'Mislukt',
  sessionInputRequired: 'Invoer nodig',
  sessionWaitingForYou: 'Wacht op jou',
  sessionStranded: 'Vastgelopen',
  sessionLoading: 'De stappen van deze sessie laden…',
  sessionPausedClosed: 'Gepauzeerd zolang het kanaal gesloten is',
  sessionPausedArchived: 'Gepauzeerd zolang het kanaal gearchiveerd is',
  sessionShowAll: 'Alle stappen tonen',
  sessionHideAll: 'Stappen verbergen',
  decisions: 'Beslissingen',
  choices: 'Keuzes',
}

export const NL_MESSAGES: Messages = {
  selectedCount: (args) => `${counted(args)} geselecteerd`,

  resultCount: ({ shown, total }) => `${shown} van ${total}`,

  cardGrabbed: ({ title }) =>
    `${title}, opgepakt. Kies een kolom met de pijltjestoetsen, zet neer met Enter, annuleer met Escape.`,
  cardAtRest: ({ title, section }) =>
    `${title}${section ? `, in ${section}` : ''}. Druk op Enter om op te pakken.`,
  cardOverTarget: ({ title, target }) =>
    `${title}, boven ${target}. Druk op Enter om neer te zetten, Escape om te annuleren.`,
  cardOverSwimlaneTarget: ({ title, target, lane }) =>
    `${title}, boven ${target} in ${lane}. Druk op Enter om neer te zetten, Escape om te annuleren.`,

  sortHandle: ({ title, position, count }) => `${title} verplaatsen, ${position} van ${count}`,
  sortGrabbed: ({ title, position, count }) =>
    `${title}, opgepakt, ${position} van ${count}. Verplaats met pijl omhoog en omlaag, zet neer met Enter, annuleer met Escape.`,
  sortDropped: ({ title, position, count }) => `${title}, neergezet op ${position} van ${count}.`,
  sortCancelled: ({ title, position, count }) =>
    `${title}, teruggezet op ${position} van ${count}.`,

  changeKind: ({ kind }) =>
    ({ added: 'Toegevoegd', changed: 'Gewijzigd', removed: 'Verwijderd' })[kind],
  changeBefore: () => 'van',
  changeAfter: () => 'naar',

  expandSection: ({ title }) => `${title} uitklappen`,
  collapseSection: ({ title }) => `${title} inklappen`,
  addToSection: ({ addLabel, section, lane }) =>
    `${addLabel} in ${section}${lane ? `, ${lane}` : ''}`,

  commentOnBlock: ({ kind }) => `Reageren op dit ${kind}`,
  commentCountOnBlock: (args) => `${counted(args)} bij dit ${args.kind}`,
  commentOnAnchor: ({ anchor }) => `Reageren op ${anchor}`,
  commentCountOnAnchor: (args) => `${counted(args)} bij ${args.anchor}`,
  sectionAnchor: ({ reference }) => `Sectie: ${reference}`,

  resizerValue: ({ pixels }) => `${pixels} pixels`,

  removeFile: ({ name }) => `${name} verwijderen`,
  fileSize: ({ bytes }) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} MB`
  },

  timeZoneNote: ({ zone }) => `Tijden zijn in ${zone}.`,

  noMatches: ({ query }) => `Niets gevonden voor “${query}”.`,

  toggleDisclosure: ({ label, open }) => `${label} ${open ? 'inklappen' : 'uitklappen'}`,

  editField: ({ label }) => `${label}, bewerken`,

  pageNumber: ({ page }) => `Pagina ${page}`,

  removeFilter: ({ label, value }) => `Filter ${label}${value ? `: ${value}` : ''} verwijderen`,

  channelMessageRemoved: ({ kind, by, reason }) =>
    (kind === 'author' ? 'Verwijderd door de schrijver' : 'Verwijderd door een moderator') +
    (by ? ` (${by})` : '') +
    (reason ? `: ${reason}` : ''),
  channelReplyTo: ({ author }) =>
    author ? `Antwoord aan ${author}` : 'Antwoord op een eerder bericht',
  channelReplyingTo: ({ author }) =>
    author ? `Je antwoordt ${author}` : 'Je antwoordt op een eerder bericht',
  channelActionPending: ({ label }) => `${label}: in afwachting`,
  channelActionDone: ({ label }) => `${label}: klaar`,
  channelTooLong: ({ max }) => `Hoogstens ${max} tekens.`,
  channelNewMessages: ({ count, type, author }) =>
    count === 1 && type
      ? `${type}${author ? ` van ${author}` : ''}`
      : `${count} ${count === 1 ? 'nieuw bericht' : 'nieuwe berichten'}`,
  channelNewNotices: ({ notices, messages }) =>
    `${[...(messages ? [messages] : []), ...notices].join('. ')}.`,
  channelSessionMoved: ({ from, to }) => `Sessie van ${from} naar ${to}`,
  channelSessionReentered: ({ state }) => `Sessie opnieuw in ${state}`,
  channelSessionWaiting: ({ labels }) => `Wacht op ${ofList(labels)}`,
  channelSessionStrandedText: ({ state }) =>
    `${state} is geen stap van deze sessie meer. Berichten verplaatsen haar niet.`,
  channelSessionHint: ({ to, guarded }) =>
    `Dit bericht zet de sessie op ${to}${guarded ? ' als aan de voorwaarde is voldaan' : ''}.`,
  channelSessionBranchHint: ({ tos }) => `Dit bericht kan de sessie op ${ofList(tos)} zetten.`,
  channelSessionOptionHint: () => ' (verplaatst de sessie)',
  channelDecisionHint: ({ to, form }) =>
    form ? ' (opent het formulier)' : to ? ` (zet de sessie op ${to})` : ' (zet de sessie verder)',
  channelChoiceHint: () => ' (plaatst dit antwoord als reactie)',
  channelSessionSummary: ({ status, label, outcome }) => {
    switch (status) {
      case 'stranded':
        return `Vastgelopen · ${label}`
      case 'final':
        return `${outcome ?? 'Beëindigd'} · ${label}`
      case 'paused':
        return `${label} · Gepauzeerd`
      case 'loading':
        return `Sessie · ${label} · Laden`
      default:
        return `Sessie · ${label}`
    }
  },
  channelSessionStep: ({ label, next, final }) =>
    [label, ...(next ? ['volgende'] : []), ...(final ? ['eindstap'] : [])].join(', '),
  channelTyping: ({ names, others }) => {
    if (others > 0) {
      return `${names.join(', ')} en ${others} ${others === 1 ? 'ander' : 'anderen'} typen…`
    }
    if (names.length === 1) return `${names[0]} typt…`
    return `${names.join(' en ')} typen…`
  },
  channelText: ({ key }) => CHANNEL_TEXT_NL[key],
  chromeText: ({ key }) => CHROME_TEXT_NL[key],
  channelUnreadCount: ({ count }) => `${count} ongelezen`,

  inProgress: ({ label }) => `${label}: bezig`,
  groupOptions: ({ label }) => `Opties voor ${label}`,
  moreCount: ({ count }) => `+${count} meer`,
  dropFiles: ({ multiple }) =>
    `Sleep ${multiple ? 'bestanden' : 'een bestand'} hierheen, of klik om te kiezen`,

  selectItem: ({ title }) => `${title} selecteren`,
  selectAllIn: ({ section }) => `Alles in ${section} selecteren`,
  moveRow: ({ title }) => `${title} verplaatsen`,
  showMoreRows: ({ count }) => `Nog ${count} tonen`,
  ofRemaining: ({ remaining }) => `van de ${remaining} resterende`,

  openCount: ({ count }) => `${count} open`,
  notOnField: ({ count }) => `${count} niet bij een veld`,
  commentsOn: ({ anchor }) => `Reacties bij ${anchor}`,
  onThisBlock: ({ kind }) => `Bij dit ${kind}`,

  editorCommand: ({ id, label }) => EDITOR_COMMANDS_NL[id] ?? label,
  commandUnavailable: ({ label }) => `${label} (hier niet beschikbaar)`,

  inboxWaitingHeading: () => 'Wacht op jou',
  inboxMentionsHeading: () => 'Vermeldingen en reacties',
  inboxUnreadHeading: () => 'Ongelezen kanalen',
  inboxWaitingEmpty: () => 'Er wacht niets op jou.',
  inboxMentionsEmpty: () => 'Geen ongelezen vermeldingen of reacties.',
  inboxUnreadEmpty: () => 'Alle kanalen zijn gelezen.',
  inboxMentioned: ({ author }) => `${author} heeft je genoemd`,
  inboxReplied: ({ author }) => `${author} heeft op je gereageerd`,
  inboxMarkRead: () => 'Markeer als gelezen',
  inboxShowing: ({ shown, total }) => `${shown} van ${total} getoond`,
  inboxDegraded: () =>
    'Sommige kanaalinstellingen konden niet worden geladen. De lijst is mogelijk onvolledig.',
}
