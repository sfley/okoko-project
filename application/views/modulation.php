<html>

<head>
<title>Modulation</title>
</head>

<body>




<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>


<figure class="highcharts-figure">
    <div id="modulation_container"></div>
    
</figure>
	
<script type="text/javascript">	





	Highcharts.chart('modulation_container', {

	 chart: {
        height: 40+'%'
    },
	title: {
        text: 'Courbe de modulation de la chaudière',
        align: 'left'
    },

    subtitle: {
        text: 'en pourcentage',
        align: 'left'
    },

    yAxis: {
        title: {
            text: 'modulation en %'
        },gridLineWidth: 1
        
    },

    xAxis: {
		categories: [<?php echo $modulation["date"] ; ?>],tickInterval:60,gridLineWidth: 1,
        tickWidth: 1
        
		
    },

    legend: {
        layout: 'vertical',
        align: 'right',
        verticalAlign: 'middle'
    },

    plotOptions: {
        series: {
            label: {
                connectorAllowed: false
            },
           
        }
    },

    series: [{
        name: 'modulation',
        data: [<?php echo $modulation["data"] ; ?>]
    
    }],

    responsive: {
        rules: [{
            condition: {
                maxWidth: 500
            },
            chartOptions: {
                legend: {
                    layout: 'horizontal',
                    align: 'center',
                    verticalAlign: 'bottom'
                }
            }
        }]
    }

});


</script>






</body>
</html>

	


  
  
