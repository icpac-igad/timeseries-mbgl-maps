# Timeseries MBGL Renderer

Generate timeseries maps using Mapbox GL Style Configuration with [mbgl-renderer](https://github.com/consbio/mbgl-renderer) Web API


# Sample output

![CDI Timeseries](examples/cdi.png)

# Features:

- Render timeseries maps collage from a layer source that accepts dynamic dates
- All other features provided by [mbgl-renderer](https://github.com/consbio/mbgl-renderer).

This program generates a series of static maps from a pre-configured sequence of dynamic date parameters for a layer source and combines them in a 2D matrix image of years and timescale. The timescale can be dekads or months for now. More date sequence support will be added

# Installation

TODO

# Usage


Make a post request to the root path with your map json configuration:

Sample json post data for ICPAC's Combined Drought Indicator

```
{
    "x_param": {
        "key": "SELECTED_MONTH",
        "options": [
            {
                "label": "Jan 01",
                "value": "01"
            },
            {
                "label": "Feb 01",
                "value": "02"
            },
            {
                "label": "Mar 01",
                "value": "03"
            },
            {
                "label": "Apr 01",
                "value": "04"
            },
            {
                "label": "May 01",
                "value": "05"
            },
            {
                "label": "Jun 01",
                "value": "06"
            },
            {
                "label": "Jul 01",
                "value": "07"
            },
            {
                "label": "Aug 01",
                "value": "08"
            },
            {
                "label": "Sep 01",
                "value": "09"
            },
            {
                "label": "Oct 01",
                "value": "10"
            },
            {
                "label": "Nov 01",
                "value": "11"
            },
            {
                "label": "Dec 01",
                "value": "12"
            }
        ]
    },
    "y_param": {
        "key": "SELECTED_YEAR",
        "options": [
            {
                "label": "2019",
                "value": "2019"
            },
            {
                "label": "2020",
                "value": "2020"
            },
            {
                "label": "2021",
                "value": "2021"
            }
        ]
    },
    "width": 100,
    "height": 100,
    "padding": 25,
    "ratio": 1,
    "center": [
        39.849808505799786,
        -3.6044446675120256
    ],
    "zoom": 5,
    "style": {
        "version": 8,
        "sprite": "https://eahazardswatch.icpac.net/tileserver-gl/styles/eahw/sprite",
        "sources": {
            "basemap": {
                "type": "raster",
                "tiles": [
                    "https://eahazardswatch.icpac.net/tileserver-gl/styles/blank_bg/{z}/{x}/{y}.png"
                ],
                "tileSize": 256
            },
            "labels": {
                "type": "raster",
                "tiles": [
                    "https://eahazardswatch.icpac.net/tileserver-gl/styles/labels_dark/{z}/{x}/{y}.png"
                ],
                "tileSize": 256
            },
            "parameter_layer": {
                "type": "raster",
                "tiles": [
                    "https://droughtwatch.icpac.net/mapserver/mukau/php/gis/mswms.php?map=mukau_w_mf&LAYERS=cdi&FORMAT=image/png&TRANSPARENT=TRUE&SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:3857&BBOX={bbox-epsg-3857}&WIDTH=256&HEIGHT=256&STYLES=&SELECTED_YEAR={SELECTED_YEAR}&SELECTED_MONTH={SELECTED_MONTH}&SELECTED_TENDAYS=01"
                ],
                "tileSize": 256
            }
        },
        "layers": [
            {
                "id": "basemap",
                "type": "raster",
                "source": "basemap"
            },
            {
                "id": "parameter_layer",
                "type": "raster",
                "source": "parameter_layer"
            },
            {
                "id": "labels",
                "type": "raster",
                "source": "labels"
            }
        ]
    }
}
```

TODO: Explain the above in detail

# Configuration

TODO: 
