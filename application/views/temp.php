<html>

<head>
<title></title>
</head>

<body>


<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>




<figure class="highcharts-figure">
    <div id="temp_container"></div>
    
</figure>
	
	

	
<script type="text/javascript">	





	Highcharts.chart('temp_container', {

	
	chart: { type:'spline',
	height: 40+'%'},
		
		title: {
        text: 'Courbes de température',
        align: 'left'
    },

    subtitle: {
        text: 'En degrés Celsius',
        align: 'left'
    },

    yAxis: {
        title: {
            text: 'température °c'
        },gridLineWidth: 1
        
    },

    xAxis: {
		categories: [<?php foreach($tempHK1 as $row) : ?><?php echo  "'".$row->Date."'".','; ?><?php endforeach; ?>],tickInterval:15,gridLineWidth: 1,
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

    series: [
	{name: 'Extérieur', 		data: [<?php foreach($tempEXT as $row) : ?><?php echo $row->Temp_EXT.','; ?><?php endforeach; ?>] },
	{name: 'Chambres', 			data: [<?php foreach($tempHK1 as $row) : ?><?php echo $row->HK1_Temp_Ambiante.','; ?><?php endforeach; ?>] }, 
	{name: 'Pièce de vie',  	data: [<?php foreach($tempHK2 as $row) : ?><?php echo $row->HK2_Temp_Ambiante.','; ?><?php endforeach; ?>] }, 
	{name: 'E.C.S',		        data: [<?php foreach($tempWW1 as $row) : ?><?php echo $row->WW1_Temp_ECS.','; ?><?php endforeach; ?>]      }
			],

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

