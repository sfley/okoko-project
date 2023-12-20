<html>

<head>
<title>test</title>
</head>

<body>




<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/highcharts-more.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/accessibility.js"></script>


<figure class="highcharts-figure">
    <div id="container_conso_graph"></div>
    <p class="highcharts-description">
      
    </p>
</figure>
	
<script type="text/javascript">	

Highcharts.chart('container_conso_graph', {

    chart: {
        type: 'gauge',
        plotBackgroundColor: null,
        plotBackgroundImage: null,
        plotBorderWidth: 0,
        plotShadow: false,
        height: '25%'
    },

    title: {
        text: 'Stock de pellets'
    },

    pane: {
        startAngle: -90,
        endAngle: 89.9,
        background: null,
        center: ['50%', '75%'],
        size: '110%'
    },

    // the value axis
    yAxis: {
        min: 0,
        max: 3000,
        tickPixelInterval: 72,
        tickPosition: 'inside',
        tickColor: Highcharts.defaultOptions.chart.backgroundColor || '#FFFFFF',
        tickLength: 20,
        tickWidth: 2,
        minorTickInterval: null,
        labels: {
            distance: 20,
            style: {
                fontSize: '14px'
            }
        },
        lineWidth: 0,
        plotBands: [{
            from: 1000,
            to: 3000,
            color: '#55BF3B', // green
            thickness: 20
        }, {
            from: 250,
            to: 1000,
            color: '#DDDF0D', // yellow
            thickness: 20
        }, {
            from: 0,
            to: 250,
            color: '#DF5353', // red
            thickness: 20
        }]
    },

    series: [{
        name: 'stock restant',
        data: [<?php echo $conso_now['data']; ?>],
        tooltip: {
            valueSuffix: ' Kg'
        },
        dataLabels: {
            format: '{y} Kg',
            borderWidth: 0,
            color: (
                Highcharts.defaultOptions.title &&
                Highcharts.defaultOptions.title.style &&
                Highcharts.defaultOptions.title.style.color
            ) || '#333333',
            style: {
                fontSize: '16px'
            }
        },
        dial: {
            radius: '80%',
            backgroundColor: 'gray',
            baseWidth: 12,
            baseLength: '0%',
            rearLength: '0%'
        },
        pivot: {
            backgroundColor: 'gray',
            radius: 6
        }

    }]

});



</script>






</body>
</html>

	


  
  
