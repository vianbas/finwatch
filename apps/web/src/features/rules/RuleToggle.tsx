import { Button } from "@/components/ui/button";
import type { Rule } from "@/lib/apiTypes";
import { toggleFailureMessage, useRuleToggle } from "@/features/rules/useRuleToggle";

interface RuleToggleProps {
  rule: Rule;
  /** Called with a display message when the toggle fails. */
  onFailure: (message: string) => void;
}

/** RuleToggle is the admin-only Enable/Disable button for one rule row. */
export function RuleToggle({ rule, onFailure }: RuleToggleProps) {
  const toggle = useRuleToggle({ onError: (error) => onFailure(toggleFailureMessage(error)) });
  const label = rule.enabled ? "Disable" : "Enable";

  return (
    <Button
      type="button"
      size="sm"
      variant="outline"
      disabled={toggle.isPending}
      aria-label={`${label} rule ${rule.name}`}
      onClick={() => toggle.mutate({ id: rule.id, enabled: !rule.enabled })}
    >
      {label}
    </Button>
  );
}
