import type { ConcreteComponent } from 'vue'

/** What Storybook's `component` field and Vue's `components` map both accept. */
type StoryComponent = Omit<ConcreteComponent<any>, 'props'>

/**
 * Registers a generic SFC with Storybook.
 *
 * A component declared `generic="T"` compiles to a generic function, which
 * neither Storybook's `component` field nor Vue's `components` map can hold.
 * Pinning the row type in `Meta<typeof C<Task>>` keeps every story's args
 * checked; this only launders the registration.
 */
export function storyComponent(component: unknown): StoryComponent {
  return component as StoryComponent
}
