import { 
  Home, 
  Router, 
  Layers, 
  Settings, 
  PanelLeft, 
  PanelRight,
  HelpCircle
} from "lucide-react";
import { cn } from "~/lib/utils";

const icons = {
  home: Home,
  router: Router,
  layers: Layers,
  settings: Settings,
  "panel-left": PanelLeft,
  "panel-right": PanelRight
};

export type IconName = keyof typeof icons;

export interface IconProps extends React.SVGProps<SVGSVGElement> {
  name: IconName;
  size?: number;
  className?: string;
}

export function Icon({ name, size = 24, className, ...props }: IconProps) {
  const IconComponent = icons[name] || HelpCircle;
  
  return (
    <IconComponent
      size={size}
      className={cn("", className)}
      {...props}
    />
  );
} 