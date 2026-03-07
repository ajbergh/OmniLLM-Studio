interface AppIconProps {
  size?: number;
  className?: string;
}

export function AppIcon({ size = 32, className }: AppIconProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 74 74"
      role="img"
      aria-label="OmniLLM-Studio"
      className={className}
    >
      <defs>
        <linearGradient id="accent-grad" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#4fd1c5" />
          <stop offset="100%" stopColor="#22d3ee" />
        </linearGradient>
      </defs>

      <rect x="0" y="0" width="74" height="74" rx="18" fill="#0f2238" stroke="#2e587c" />

      <circle cx="22" cy="26" r="6" fill="#4fd1c5" />
      <circle cx="38" cy="26" r="6" fill="#22d3ee" />
      <circle cx="30" cy="42" r="6" fill="#67e8f9" />
      <circle cx="52" cy="42" r="6" fill="#a5f3fc" />

      <path
        d="M22 26 L38 26 M22 26 L30 42 M38 26 L52 42 M30 42 L52 42"
        stroke="url(#accent-grad)"
        strokeWidth="1.6"
        strokeLinecap="round"
        opacity="0.85"
      />

      <rect x="14" y="55" width="46" height="4" rx="2" fill="#4fd1c5" opacity="0.35" />
    </svg>
  );
}
