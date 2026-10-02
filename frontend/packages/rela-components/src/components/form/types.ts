/** One choice in a select or multi-select. */
export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
  /**
   * A line under the label, explaining what the choice means.
   *
   * Only drawn where the control has room for it, which today is
   * `RlRadioGroup`. A native `select` cannot render it and a multi-select
   * option row would grow past what its panel can show, so both ignore it
   * rather than truncating something the caller thought was being read.
   */
  description?: string
}
