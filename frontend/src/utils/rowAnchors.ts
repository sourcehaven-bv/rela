import type { ViewSection, ViewTreeNode } from '@/api/views'

/**
 * Maps each entity id on a view page to the section whose row carries its
 * `#<id>` anchor (TKT-QO14GB). An entity can appear in several sections, and
 * a DOM id must be unique, so only its first row in page order gets the id.
 */
export function anchorSections(sections: readonly ViewSection[]): Map<string, string> {
  const first = new Map<string, string>()
  const claim = (id: string, sectionId: string) => {
    if (!first.has(id)) first.set(id, sectionId)
  }
  const walk = (nodes: readonly ViewTreeNode[] | undefined, sectionId: string) => {
    for (const node of nodes ?? []) {
      claim(node.entity.id, sectionId)
      walk(node.children, sectionId)
    }
  }
  for (const s of sections) {
    for (const e of s.entities ?? []) claim(e.id, s.sectionId)
    for (const r of s.rows ?? []) claim(r.entityId, s.sectionId)
    for (const g of s.groups ?? []) {
      for (const r of g.rows ?? []) claim(r.entityId, s.sectionId)
      for (const e of g.entities ?? []) claim(e.id, s.sectionId)
    }
    walk(s.tree, s.sectionId)
  }
  return first
}
