import _ from "lodash";
import { HelmChart } from "../../helmchart";
import { Context } from "../../context";
import { Stores } from "../../schema/stores";

export function AppsQueries(stores: Stores) {
  return {
    async listApps(root: any, args: any, context: Context) {
      return {
        watches: async () => (await stores.watchStore.listWatches(context.session.userId)).map(watch => watch.toSchema(root, stores, context)),
        kotsApps: async () => {
          // TODO: Get kots apps from database
          return [{
            id: "kots-app"
          }]
        },
        pendingUnforks: async () => {
          const clusters = await stores.clusterStore.listClusters(context.session.userId);
          let helmCharts: HelmChart[] = [];
          for (const cluster of clusters) {
            const clusterCharts = await stores.helmChartStore.listHelmChartsInCluster(cluster.id);
            helmCharts = helmCharts.concat(clusterCharts);
          }
          return helmCharts.map(chart => chart.toSchema());
        }
      };
    },

    // async searchApps(root: any, args: any, context: Context): Promise<Watch[]> {
    //   const watches = await stores.watchStore.searchWatches(context.session.userId, args.watchName);
    //   return watches.map(watch => watch.toSchema(root, stores, context));
    // },

  }
}
