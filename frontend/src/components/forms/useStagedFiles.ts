import { shallowRef, type Ref, type ShallowRef } from 'vue'
import { uploadAttachment, attachmentErrorReason } from '@/api/attachments'
import { t } from '@/i18n'

/** A staged file the server refused, with the reason it gave. */
export interface StagedFailure {
  file: string
  message: string
}

/** What the staged-file state needs from the form that owns it. */
export interface StagedFilesHost {
  errors: Ref<Record<string, string>>
  onChange: () => void
  isSkipped: (property: string) => boolean
}

/** The create-mode staged-file state and the operations on it. */
export interface StagedFiles {
  stagedFiles: ShallowRef<Record<string, File[]>>
  updateStagedFiles: (property: string, files: File[]) => void
  hasStagedFiles: () => boolean
  uploadStagedFiles: (entityType: string, entityId: string) => Promise<StagedFailure[]>
  stagedFailureMessage: (failures: StagedFailure[]) => string
}

/**
 * Files picked in create mode, uploaded once the server returns an id
 * (TKT-7K3BJF): an attachment cannot exist before its row. They stay out of
 * `formData` so no File is POSTed as a property value. A shallowRef keeps
 * File identity the same in tests and production (RR-C1MI2N). Staging a file
 * clears its field's validation error (BUG-L1DHC5). Upload is sequential and
 * continues on error (RR-Z7C3CY), and skips properties the wizard hid
 * (RR-OI6P51). The failure message names every file that failed.
 */
export function useStagedFiles(host: StagedFilesHost): StagedFiles {
  const stagedFiles = shallowRef<Record<string, File[]>>({})

  function updateStagedFiles(property: string, files: File[]) {
    if (files.length) stagedFiles.value = { ...stagedFiles.value, [property]: files }
    else {
      const next = { ...stagedFiles.value }
      delete next[property]
      stagedFiles.value = next
    }
    if (files.length && host.errors.value[property]) {
      const next = { ...host.errors.value }
      delete next[property]
      host.errors.value = next
    }
    host.onChange()
  }

  function hasStagedFiles(): boolean {
    return Object.values(stagedFiles.value).some((list) => list.length > 0)
  }

  async function uploadStagedFiles(entityType: string, entityId: string): Promise<StagedFailure[]> {
    const failures: StagedFailure[] = []
    for (const [property, files] of Object.entries(stagedFiles.value)) {
      if (host.isSkipped(property)) continue
      for (const file of files) {
        try {
          await uploadAttachment(entityType, entityId, property, file)
        } catch (err) {
          failures.push({ file: file.name, message: attachmentErrorReason(err) })
        }
      }
    }
    return failures
  }

  function stagedFailureMessage(failures: StagedFailure[]): string {
    const detail = failures.map((f) => `${f.file} (${f.message})`).join(', ')
    return t('form.attachFailed', { n: failures.length, detail }, failures.length)
  }

  return { stagedFiles, updateStagedFiles, hasStagedFiles, uploadStagedFiles, stagedFailureMessage }
}
