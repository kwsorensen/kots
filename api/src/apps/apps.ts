import { Watch } from "../watch";
import { KotsApp } from "../kots_app";
import { HelmChart } from "../helmchart";

export interface Apps {
  watches?: Array<Watch>;
  kotsApps?: Array<KotsApp>;
  pendingUnforks?: Array<HelmChart>;
}
